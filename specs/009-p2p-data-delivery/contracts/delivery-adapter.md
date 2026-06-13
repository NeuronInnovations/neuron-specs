# API Contract: DeliveryAdapter Interface

**Source**: spec.md FR-D01–D06

---

## Interface

```
DeliveryAdapter {
    connect(peerID: PeerID, multiaddrs: []string, protocol: string, opts?: ConnectOptions) → (DeliveryChannel, error)
    send(channel: DeliveryChannel, data: []byte) → (SendResult, error)
    receive(channel: DeliveryChannel) → (AsyncStream<DataFrame>, error)
    disconnect(channel: DeliveryChannel) → error
    getStatus(channel: DeliveryChannel) → ChannelStatus
}
```

## Operations

### connect (FR-D02)
Establishes a delivery channel. Attempts direct dial first; falls back to relay if direct fails and counterparty natStatus is "private"/"unknown". Returns DeliveryChannel handle.

### send (FR-D03)
Transmits a length-prefixed data frame (FR-D22). Data is opaque bytes. Returns bytesSent count.

### receive (FR-D04)
Returns async stream of DataFrames. Each frame includes data bytes and receivedAt timestamp. Keep-alive frames (zero-length) are consumed silently (FR-D24).

### disconnect (FR-D05)
Closes channel gracefully. Both sides notified. Subsequent operations return ChannelClosed error.

### getStatus (FR-D06)
Returns current ConnectionState + transport identifier. No side effects.

## ConnectOptions (optional)

```
ConnectOptions {
    natStatus?: string       // "public", "private", "unknown"
    backoffConfig?: BackoffConfig
    relayAddrs?: []string    // static relay multiaddrs
}
```

## SendResult

```
SendResult {
    bytesSent: int
}
```
