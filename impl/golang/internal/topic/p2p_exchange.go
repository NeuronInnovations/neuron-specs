package topic

import "fmt"

// FR-T17: neuron-p2p-exchange service type
// NeuronP2PExchangeServiceDef is the Spec 004 representation of a neuron-p2p-exchange
// service entry as found in an agentURI JSON document. Each entry links a libp2p peer
// to a topic via a TopicRef URI for cross-reference validation.
type NeuronP2PExchangeServiceDef struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Version  string `json:"version"`
	PeerID   string `json:"peerID"`
	Protocol string `json:"protocol"`
	TopicRef string `json:"topicRef"`
}

// FR-T18: Cross-reference validation (topicRef -> existing neuron-topic)
// ValidateCrossReferences checks that every NeuronP2PExchangeServiceDef.TopicRef
// points to a valid topic service in the provided topics slice.
// A TopicRef is considered valid if it can be parsed as a URI and matches the URI
// of at least one topic service.
//
// Returns ErrBrokenTopicRef if any P2P exchange service references a topic
// that does not exist in the topics slice.
func ValidateCrossReferences(topics []NeuronTopicServiceDef, p2p []NeuronP2PExchangeServiceDef) error {
	// Build a set of known topic URIs from the topic services.
	knownURIs := make(map[string]bool, len(topics))
	for _, svc := range topics {
		ref, err := ExtractTopicRef(svc)
		if err != nil {
			continue
		}
		knownURIs[ref.URI()] = true
	}

	// Validate that each P2P exchange service references a known topic.
	for _, p2pSvc := range p2p {
		if p2pSvc.TopicRef == "" {
			return NewTopicError(ErrBrokenTopicRef,
				fmt.Sprintf("p2p exchange service %q has empty topicRef", p2pSvc.Name))
		}

		// Parse the TopicRef URI to normalize it.
		ref, err := TopicRefFromURI(p2pSvc.TopicRef)
		if err != nil {
			return WrapTopicError(ErrBrokenTopicRef,
				fmt.Sprintf("p2p exchange service %q has invalid topicRef: %s", p2pSvc.Name, p2pSvc.TopicRef), err)
		}

		if !knownURIs[ref.URI()] {
			return NewTopicError(ErrBrokenTopicRef,
				fmt.Sprintf("p2p exchange service %q references unknown topic: %s", p2pSvc.Name, p2pSvc.TopicRef))
		}
	}

	return nil
}
