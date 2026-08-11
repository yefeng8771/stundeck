package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// The fw4 gateway mode covers the deployment where StunDeck runs directly on
// an OpenWrt router. There is no upstream IGD to talk to (and under CGNAT the
// carrier NAT never answers UPnP/NAT-PMP anyway), so the port has to be opened
// on the local firewall instead.
//
// Rules are inserted into the running fw4 ruleset only. Nothing is written to
// /etc/config/firewall, so a `fw4 reload` drops them; reassertFirewall puts
// them back. That keeps the persistent configuration clean and makes an
// uninstall a no-op.
const (
	firewallTable        = "fw4"
	firewallFamily       = "inet"
	firewallInputChain   = "input_wan"
	firewallCommentTag   = "stundeck"
	firewallRuleBinary   = "nft"
	firewallReassertTick = 60
)

type firewallRule struct {
	Handle  uint64 `json:"handle"`
	Comment string `json:"comment"`
}

type firewallListing struct {
	Nftables []struct {
		Rule *firewallRule `json:"rule"`
	} `json:"nftables"`
}

// runNFT feeds the script through stdin instead of passing it as arguments.
// nft re-joins argv into a single string before parsing it, so a comment
// containing ":" is rejected as a syntax error unless the quotes survive to
// the parser. Only a script (file or stdin) keeps them.
func runNFT(ctx context.Context, script string) (string, error) {
	binary, err := exec.LookPath(firewallRuleBinary)
	if err != nil {
		return "", fmt.Errorf("nft binary is unavailable: %w", err)
	}
	command := exec.CommandContext(ctx, binary, "-f", "-")
	command.Stdin = strings.NewReader(script + "\n")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, firstFirewallLine(message))
		}
		return "", err
	}
	return stdout.String(), nil
}

func firstFirewallLine(message string) string {
	if index := strings.IndexByte(message, '\n'); index >= 0 {
		return strings.TrimSpace(message[:index])
	}
	return message
}

// firewallProtocol keeps the value that reaches the nft script constrained to
// a fixed vocabulary; it is never interpolated from free-form input.
func firewallProtocol(protocol string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "tcp":
		return "tcp", nil
	case "udp":
		return "udp", nil
	default:
		return "", fmt.Errorf("unsupported firewall protocol %q", protocol)
	}
}

// firewallCommentToken strips everything outside [A-Za-z0-9_-] so a service ID
// can never terminate the quoted comment and inject extra nft statements.
func firewallCommentToken(value string) string {
	var builder strings.Builder
	for _, symbol := range value {
		switch {
		case symbol >= 'a' && symbol <= 'z',
			symbol >= 'A' && symbol <= 'Z',
			symbol >= '0' && symbol <= '9',
			symbol == '_', symbol == '-':
			builder.WriteRune(symbol)
		}
	}
	return builder.String()
}

func firewallCommentPrefix(serviceID string) string {
	return firewallCommentTag + ":" + firewallCommentToken(shortServiceID(serviceID)) + ":"
}

func firewallComment(mapping GatewayMapping) (string, error) {
	protocol, err := firewallProtocol(mapping.Protocol)
	if err != nil {
		return "", err
	}
	if mapping.ExternalPort < 1 || mapping.ExternalPort > 65535 {
		return "", fmt.Errorf("invalid firewall port %d", mapping.ExternalPort)
	}
	return fmt.Sprintf("%s%s:%d", firewallCommentPrefix(mapping.ServiceID), protocol, mapping.ExternalPort), nil
}

func applyFirewallMapping(ctx context.Context, mapping GatewayMapping) error {
	comment, err := firewallComment(mapping)
	if err != nil {
		return err
	}
	protocol, err := firewallProtocol(mapping.Protocol)
	if err != nil {
		return err
	}
	// Drop stale entries for this service first: the NATMap bind port drifts
	// between restarts, so an earlier port would otherwise stay open.
	if err := pruneFirewallRules(ctx, firewallCommentPrefix(mapping.ServiceID), comment); err != nil {
		return err
	}
	existing, err := findFirewallHandles(ctx, comment, true)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	script := fmt.Sprintf(
		`insert rule %s %s %s meta nfproto ipv4 %s dport %d counter accept comment "%s"`,
		firewallFamily, firewallTable, firewallInputChain, protocol, mapping.ExternalPort, comment,
	)
	if _, err := runNFT(ctx, script); err != nil {
		return err
	}
	return nil
}

func removeFirewallMapping(ctx context.Context, mapping GatewayMapping) error {
	comment, err := firewallComment(mapping)
	if err != nil {
		return err
	}
	return deleteFirewallHandles(ctx, comment, true)
}

func pruneFirewallRules(ctx context.Context, prefix string, keep string) error {
	handles, err := findFirewallHandles(ctx, prefix, false)
	if err != nil {
		return err
	}
	keepHandles, err := findFirewallHandles(ctx, keep, true)
	if err != nil {
		return err
	}
	protected := make(map[uint64]bool, len(keepHandles))
	for _, handle := range keepHandles {
		protected[handle] = true
	}
	var stale []uint64
	for _, handle := range handles {
		if !protected[handle] {
			stale = append(stale, handle)
		}
	}
	return deleteFirewallHandleList(ctx, stale)
}

func deleteFirewallHandles(ctx context.Context, match string, exact bool) error {
	handles, err := findFirewallHandles(ctx, match, exact)
	if err != nil {
		return err
	}
	return deleteFirewallHandleList(ctx, handles)
}

func deleteFirewallHandleList(ctx context.Context, handles []uint64) error {
	var failures []string
	for _, handle := range handles {
		script := fmt.Sprintf("delete rule %s %s %s handle %d",
			firewallFamily, firewallTable, firewallInputChain, handle)
		if _, err := runNFT(ctx, script); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// findFirewallHandles reads the chain as JSON so handles are taken from a
// structured field rather than scraped out of the human-readable output.
func findFirewallHandles(ctx context.Context, match string, exact bool) ([]uint64, error) {
	if match == "" {
		return nil, nil
	}
	binary, err := exec.LookPath(firewallRuleBinary)
	if err != nil {
		return nil, fmt.Errorf("nft binary is unavailable: %w", err)
	}
	command := exec.CommandContext(ctx, binary, "-j", "-a", "list", "chain",
		firewallFamily, firewallTable, firewallInputChain)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("list firewall chain: %w: %s", err, firstFirewallLine(message))
		}
		return nil, fmt.Errorf("list firewall chain: %w", err)
	}
	var listing firewallListing
	if err := json.Unmarshal(stdout.Bytes(), &listing); err != nil {
		return nil, fmt.Errorf("decode firewall chain: %w", err)
	}
	var handles []uint64
	for _, entry := range listing.Nftables {
		if entry.Rule == nil || entry.Rule.Comment == "" {
			continue
		}
		if exact {
			if entry.Rule.Comment == match {
				handles = append(handles, entry.Rule.Handle)
			}
			continue
		}
		if strings.HasPrefix(entry.Rule.Comment, match) {
			handles = append(handles, entry.Rule.Handle)
		}
	}
	return handles, nil
}

// reassertFirewall re-adds the rule if something else flushed it. A
// `fw4 reload` (triggered by any unrelated firewall or network config change)
// rebuilds the ruleset from /etc/config/firewall and silently drops our
// runtime-only rule, which would close the port without any error surfacing.
func (m *Manager) reassertFirewall(ctx context.Context, serviceID string, mapping GatewayMapping, generation uint64) {
	ticker := time.NewTicker(firewallReassertTick * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			running := m.processes[serviceID]
			current := running != nil && running.gatewayGeneration == generation && running.gateway != nil && sameGatewayMapping(*running.gateway, mapping)
			m.mu.Unlock()
			if !current {
				return
			}
			applyCtx, cancel := context.WithTimeout(ctx, gatewayTimeout)
			err := applyFirewallMapping(applyCtx, mapping)
			cancel()
			if err != nil {
				m.logger.Warn("reassert fw4 rule", "service_id", serviceID, "error", err)
				_ = m.store.SetServiceRuntime(context.Background(), serviceID, "gateway_error", "fw4 rule reassert failed: "+err.Error(), true)
			}
		}
	}
}

func firewallAvailable() error {
	if _, err := exec.LookPath(firewallRuleBinary); err != nil {
		return errors.New("未找到 nft 命令，fw4 模式仅支持 OpenWrt 22.03 及以上的 firewall4")
	}
	return nil
}

// firewallChainPresent reports whether the fw4 input_wan chain exists, which
// is what distinguishes a firewall4 router from a plain nftables host.
func firewallChainPresent(ctx context.Context) error {
	if err := firewallAvailable(); err != nil {
		return err
	}
	if _, err := findFirewallHandles(ctx, firewallCommentTag+":", false); err != nil {
		return err
	}
	return nil
}
