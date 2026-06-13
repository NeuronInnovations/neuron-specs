package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/neuron-sdk/neuron-go-sdk/internal/health"
	"github.com/neuron-sdk/neuron-go-sdk/internal/keylib"
	"github.com/neuron-sdk/neuron-go-sdk/internal/topic"
)

// --- Main Test Script ---

func main() {
	fmt.Println("=== Neuron SDK — Live HCS Heartbeat Publishing Test ===")
	fmt.Println()

	// --- Step 1: Initialize Hedera testnet client ---
	fmt.Println("[1/7] Initializing Hedera testnet client...")

	client, operatorID, err := topic.NewTestnetClientFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		fmt.Fprintf(os.Stderr,
			"  Set %s (e.g. 0.0.X) and %s (ECDSA secp256k1 hex)\n"+
				"  before running this live HCS test. Operator credentials\n"+
				"  must never be hardcoded in source.\n",
			topic.HederaOperatorEnvAccountID, topic.HederaOperatorEnvPrivateKey)
		os.Exit(1)
	}

	fmt.Printf("  Operator: %s\n", operatorID.String())
	fmt.Println("  Network:  Hedera Testnet")
	fmt.Println()

	// --- Step 2: Create HCS topic ---
	fmt.Println("[2/7] Creating HCS topic on testnet...")

	hcsClient := topic.NewRealHCSClient(client)
	adapter := topic.NewHCSAdapter(hcsClient)

	topicIdStr, err := hcsClient.CreateTopic("neuron-heartbeat-test")
	if err != nil {
		fatal("create HCS topic", err)
	}
	fmt.Printf("  Topic ID: %s\n", topicIdStr)
	fmt.Println()

	// --- Step 3: Generate Neuron secp256k1 key ---
	fmt.Println("[3/7] Generating Neuron secp256k1 identity key...")

	neuronKey, err := keylib.NewNeuronPrivateKey()
	if err != nil {
		fatal("generate Neuron key", err)
	}

	senderAddr := neuronKey.PublicKey().EVMAddress().Hex()
	fmt.Printf("  EVM Address: %s\n", senderAddr)
	fmt.Printf("  Public Key:  %s\n", neuronKey.PublicKey().Hex())
	fmt.Println()

	// --- Step 4: Build HeartbeatPayload ---
	fmt.Println("[4/7] Building HeartbeatPayload...")

	now := uint64(time.Now().Unix())
	deadline := now + 60 // 60 seconds from now

	payload := health.BuildHeartbeatPayload(deadline, health.RoleBuyer,
		health.WithCapabilities(&health.Capabilities{
			NATReachability: true,
			NATType:         health.NATNone,
			Protocols:       []health.ProtocolID{"/neuron/heartbeat/1.0.0"},
		}),
	)

	payloadJSON, _ := json.Marshal(payload)
	fmt.Printf("  Payload JSON (%d bytes): %s\n", len(payloadJSON), string(payloadJSON))
	fmt.Printf("  Deadline:    %d (now + 60s)\n", deadline)
	fmt.Printf("  SenderClock: %d\n", now)
	fmt.Println()

	// --- Step 5: Publish heartbeat via full pipeline ---
	fmt.Println("[5/7] Publishing heartbeat via PublishHeartbeat()...")
	fmt.Println("  Pipeline: ValidateOutbound -> TrimPayload -> Serialize -> Sign -> HCSAdapter.Publish")

	stdOutRef, err := topic.NewTopicRef(topic.BackendHCS, topicIdStr)
	if err != nil {
		fatal("create TopicRef", err)
	}

	result, err := health.PublishHeartbeat(payload, &neuronKey, stdOutRef, adapter, now, 1)
	if err != nil {
		fatal("PublishHeartbeat", err)
	}

	fmt.Printf("  Transaction ID: %s\n", result.TransactionRef)
	fmt.Printf("  Confirmed:      %v\n", result.Confirmed)
	fmt.Println()

	// --- Step 6: Schedule next heartbeat ---
	fmt.Println("[6/7] Computing next heartbeat schedule...")

	nextSend := health.ScheduleNextHeartbeat(result, 60, now)
	fmt.Printf("  Next heartbeat at: %d (in %d seconds)\n", nextSend, nextSend-now)
	fmt.Println()

	// --- Step 7: Verify on Hedera mirror node ---
	fmt.Println("[7/7] Querying Hedera mirror node to verify published message...")
	fmt.Println("  Waiting 8 seconds for mirror node propagation...")
	time.Sleep(8 * time.Second)

	mirrorURL := fmt.Sprintf(
		"https://testnet.mirrornode.hedera.com/api/v1/topics/%s/messages?limit=1&order=desc",
		topicIdStr,
	)
	fmt.Printf("  Mirror URL: %s\n", mirrorURL)

	resp, err := http.Get(mirrorURL)
	if err != nil {
		fmt.Printf("  WARNING: Mirror node query failed: %v\n", err)
		fmt.Println("  You can manually check the URL above.")
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 200 {
			var mirrorResp struct {
				Messages []struct {
					ConsensusTimestamp string `json:"consensus_timestamp"`
					Message            string `json:"message"`
					SequenceNumber     int    `json:"sequence_number"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(body, &mirrorResp); err != nil {
				fmt.Printf("  WARNING: Failed to parse mirror response: %v\n", err)
				fmt.Printf("  Raw response: %s\n", string(body))
			} else if len(mirrorResp.Messages) > 0 {
				msg := mirrorResp.Messages[0]
				fmt.Printf("  Consensus Timestamp: %s\n", msg.ConsensusTimestamp)
				fmt.Printf("  Sequence Number:     %d\n", msg.SequenceNumber)

				// Decode the base64 message.
				decoded, err := base64.StdEncoding.DecodeString(msg.Message)
				if err != nil {
					fmt.Printf("  WARNING: Failed to decode base64 message: %v\n", err)
				} else {
					fmt.Printf("  Raw Message (%d bytes): %s\n", len(decoded), string(decoded))

					// Parse as TopicMessage to extract the heartbeat payload.
					var topicMsg topic.TopicMessage
					if err := json.Unmarshal(decoded, &topicMsg); err != nil {
						fmt.Printf("  WARNING: Failed to parse TopicMessage: %v\n", err)
					} else {
						signature := topicMsg.Signature()
						payloadBytes := topicMsg.Payload()
						fmt.Printf("  Sender Address: %s\n", topicMsg.SenderAddress())
						if len(signature) >= 8 {
							fmt.Printf("  Signature:      %x... (%d bytes)\n", signature[:8], len(signature))
						} else {
							fmt.Printf("  Signature:      %x (%d bytes)\n", signature, len(signature))
						}
						fmt.Printf("  Timestamp:      %d\n", topicMsg.Timestamp())
						fmt.Printf("  Seq Number:     %d\n", topicMsg.SequenceNumber())

						// Decode the heartbeat payload.
						var hbPayload health.HeartbeatPayload
						if err := json.Unmarshal(payloadBytes, &hbPayload); err != nil {
							fmt.Printf("  WARNING: Failed to parse HeartbeatPayload: %v\n", err)
							fmt.Printf("  Raw Payload: %s\n", string(payloadBytes))
						} else {
							fmt.Println()
							fmt.Println("  === VERIFIED HEARTBEAT PAYLOAD ===")
							fmt.Printf("  Type:     %s\n", hbPayload.Type)
							fmt.Printf("  Version:  %s\n", hbPayload.Version)
							fmt.Printf("  Deadline: %d\n", hbPayload.NextHeartbeatDeadline)
							fmt.Printf("  Role:     %s\n", hbPayload.Role)
							if hbPayload.Capabilities != nil {
								fmt.Printf("  NAT:      %s (reachable: %v)\n", hbPayload.Capabilities.NATType, hbPayload.Capabilities.NATReachability)
							}
						}
					}
				}
			} else {
				fmt.Println("  No messages found yet. Try querying manually in a few seconds:")
				fmt.Printf("  curl '%s'\n", mirrorURL)
			}
		} else {
			fmt.Printf("  Mirror node returned HTTP %d: %s\n", resp.StatusCode, string(body))
		}
	}

	fmt.Println()
	fmt.Println("=== Test Complete ===")
	fmt.Printf("Topic ID:     %s\n", topicIdStr)
	fmt.Printf("Transaction:  %s\n", result.TransactionRef)
	fmt.Printf("Sender:       %s\n", senderAddr)
	fmt.Printf("Mirror URL:   %s\n", mirrorURL)

	os.Exit(0)
}

func fatal(context string, err error) {
	fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", context, err)
	os.Exit(1)
}
