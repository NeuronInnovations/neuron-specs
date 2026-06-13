package payment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- T020 (continued): Escrow Payload JSON Tests ---

func TestEscrowCreated_JSON_RoundTrip(t *testing.T) {
	ec := EscrowCreated{
		Type: PayloadEscrowCreated, Version: "1.0.0", RequestID: "id",
		EscrowRef: "evm-escrow:296:0xContract", DepositAmount: "10", DepositCurrency: "USDC",
	}

	data, err := json.Marshal(ec)
	require.NoError(t, err)

	var ec2 EscrowCreated
	err = json.Unmarshal(data, &ec2)
	require.NoError(t, err)

	data2, err := json.Marshal(ec2)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(data2))
}

func TestEscrowCreated_JSON_FieldOrder(t *testing.T) {
	ec := EscrowCreated{
		Type: PayloadEscrowCreated, Version: "1.0.0", RequestID: "id",
		EscrowRef: "ref", DepositAmount: "1", DepositCurrency: "USDC",
	}

	data, err := json.Marshal(ec)
	require.NoError(t, err)
	s := string(data)

	assert.Greater(t, findJSONKeyIndex(s, "escrowRef"), findJSONKeyIndex(s, "requestId"))
	assert.Greater(t, findJSONKeyIndex(s, "depositAmount"), findJSONKeyIndex(s, "escrowRef"))
	assert.Greater(t, findJSONKeyIndex(s, "depositCurrency"), findJSONKeyIndex(s, "depositAmount"))
}

func TestInvoice_JSON_RoundTrip(t *testing.T) {
	inv := Invoice{
		Type: PayloadInvoice, Version: "1.0.0", RequestID: "id",
		ReleaseRequestRef: "ref1", EscrowRef: "ref2",
		Amount: "10", Currency: "USDC", Period: "PT1H",
	}

	data, err := json.Marshal(inv)
	require.NoError(t, err)

	var inv2 Invoice
	err = json.Unmarshal(data, &inv2)
	require.NoError(t, err)

	data2, err := json.Marshal(inv2)
	require.NoError(t, err)
	assert.Equal(t, string(data), string(data2))
}

func TestInvoiceAck_JSON_RoundTrip(t *testing.T) {
	t.Run("approved without optionals", func(t *testing.T) {
		ack := InvoiceAck{
			Type: PayloadInvoiceAck, Version: "1.0.0", RequestID: "id",
			ReleaseRequestRef: "ref", Action: "approved",
		}
		data, err := json.Marshal(ack)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "depositedMore")
		assert.NotContains(t, string(data), "newBalance")

		var ack2 InvoiceAck
		err = json.Unmarshal(data, &ack2)
		require.NoError(t, err)

		data2, err := json.Marshal(ack2)
		require.NoError(t, err)
		assert.Equal(t, string(data), string(data2))
	})

	t.Run("approved with optionals", func(t *testing.T) {
		deposited := true
		ack := InvoiceAck{
			Type: PayloadInvoiceAck, Version: "1.0.0", RequestID: "id",
			ReleaseRequestRef: "ref", Action: "approved",
			DepositedMore: &deposited, NewBalance: "20",
		}
		data, err := json.Marshal(ack)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"depositedMore":true`)
		assert.Contains(t, string(data), `"newBalance":"20"`)
	})

	t.Run("refused", func(t *testing.T) {
		ack := InvoiceAck{
			Type: PayloadInvoiceAck, Version: "1.0.0", RequestID: "id",
			ReleaseRequestRef: "ref", Action: "refused",
		}
		data, err := json.Marshal(ack)
		require.NoError(t, err)
		assert.Contains(t, string(data), `"action":"refused"`)
	})
}
