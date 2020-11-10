// +build !confonly

package dispatcher

import (
	"context"
	"v2ray.com/core/common"
	"v2ray.com/core/common/protocol/bittorrent"
	"v2ray.com/core/common/protocol/http"
	"v2ray.com/core/common/protocol/tls"
)

type SniffResult interface {
	Protocol() string
	Domain() string
}

type protocolSniffer func([]byte, context.Context) (SniffResult, error)

type protocolSnifferWithMetadata struct {
	protocolSniffer protocolSniffer
	// A Metadata sniffer will be invoked on connection establishment only, with nil body,
	// for both TCP and UDP connections
	// It will not be shown as a traffic type for routing.
	metadataSniffer bool
}

type Sniffer struct {
	sniffer []protocolSnifferWithMetadata
}

func NewSniffer() *Sniffer {
	return &Sniffer{
		sniffer: []protocolSnifferWithMetadata{
			{func(b []byte, c context.Context) (SniffResult, error) { return http.SniffHTTP(b) }, false},
			{func(b []byte, c context.Context) (SniffResult, error) { return tls.SniffTLS(b) }, false},
			{func(b []byte, c context.Context) (SniffResult, error) { return bittorrent.SniffBittorrent(b) }, false},
		},
	}
}

var errUnknownContent = newError("unknown content")

func (s *Sniffer) Sniff(payload []byte, c context.Context) (SniffResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		s := si.protocolSniffer
		if si.metadataSniffer {
			continue
		}
		result, err := s(payload, c)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		s.sniffer = pendingSniffer
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func (s *Sniffer) SniffMetadata(c context.Context) (SniffResult, error) {
	var pendingSniffer []protocolSnifferWithMetadata
	for _, si := range s.sniffer {
		s := si.protocolSniffer
		if !si.metadataSniffer {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}
		result, err := s(nil, c)
		if err == common.ErrNoClue {
			pendingSniffer = append(pendingSniffer, si)
			continue
		}

		if err == nil && result != nil {
			return result, nil
		}
	}

	if len(pendingSniffer) > 0 {
		s.sniffer = pendingSniffer
		return nil, common.ErrNoClue
	}

	return nil, errUnknownContent
}

func CompositeResult(domainResult SniffResult, protocolResult SniffResult) SniffResult {
	return &compositeResult{domainResult: domainResult, protocolResult: protocolResult}
}

type compositeResult struct {
	domainResult   SniffResult
	protocolResult SniffResult
}

func (c compositeResult) Protocol() string {
	return c.protocolResult.Protocol()
}

func (c compositeResult) Domain() string {
	return c.domainResult.Domain()
}
