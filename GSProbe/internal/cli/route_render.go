package cli

import "strings"

func routePathNodes(r routeRow) []routeMapNode {
	if len(r.Path) > 0 {
		return r.Path
	}
	if len(r.Labels) == 0 {
		return nil
	}
	nodes := make([]routeMapNode, 0, len(r.Labels))
	for _, label := range r.Labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		nodes = append(nodes, routeMapNode{Label: label})
	}
	return nodes
}

func routeNodeText(n routeMapNode) string {
	label := strings.TrimSpace(n.Label)
	region := strings.TrimSpace(n.Region)
	if label == "" {
		return "—"
	}
	if region != "" {
		return "[" + region + "]" + label
	}
	return label
}

func routeMapText(r routeRow) string {
	nodes := routePathNodes(r)
	if len(nodes) == 0 {
		line := strings.TrimSpace(routeLineText(r))
		if line == "" || strings.EqualFold(line, "unknown") || line == "—" {
			return paint(colorMuted, "未知")
		}
		return colorRouteMapLine(line)
	}
	parts := []string{paint(colorAccent, "本机")}
	for _, n := range nodes {
		parts = append(parts, colorRouteLabel(routeNodeText(n)))
	}
	return strings.Join(parts, paint(colorDim, " → "))
}

func colorRouteMapLine(line string) string {
	segs := strings.Split(line, "→")
	var parts []string
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if len(parts) > 0 {
			parts = append(parts, paint(colorDim, " → "))
		}
		parts = append(parts, colorRouteLabel(seg))
	}
	if len(parts) == 0 {
		return paint(colorMuted, "未知")
	}
	return strings.Join(parts, "")
}
