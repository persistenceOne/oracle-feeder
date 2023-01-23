package oracle

import (
	"context"
	"net"
	"strings"
)

const (
	protocolStr = "tcp"
)

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return connect(addr)
}

// connect dials the given address and returns a net.Conn. The protoAddr
// argument should be prefixed with the protocol,
// eg. "tcp://127.0.0.1:8080" or "unix:///tmp/test.sock".
func connect(protoAddr string) (net.Conn, error) {
	proto, address := protocolAndAddress(protoAddr)
	conn, err := net.Dial(proto, address)
	return conn, err
}

// protocolAndAddress splits an address into the protocol and address components.
// For instance, "tcp://127.0.0.1:8080" will be split into "tcp" and "127.0.0.1:8080".
// If the address has no protocol prefix, the default is "tcp".
func protocolAndAddress(listenAddr string) (string, string) {
	protocol, address := protocolStr, listenAddr

	parts := strings.SplitN(address, "://", 2) //nolint:gomnd //no need to make const
	if len(parts) == 2 {                       //nolint:gomnd //no need to make const
		protocol, address = parts[0], parts[1]
	}

	return protocol, address
}
