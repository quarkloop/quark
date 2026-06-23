package output

import (
        "fmt"
        "io"
        "os"
        "strings"
        "time"

        "github.com/fatih/color"
        "github.com/olekukonko/tablewriter"
        "github.com/quarkloop/quark/cli/internal/model"
)

// PrettyPrinter renders API responses as colored tables and aligned columns.
// This is the default printer for interactive terminal use.
type PrettyPrinter struct {
        w io.Writer
}

func newTable(w io.Writer, headers []string) *tablewriter.Table {
        t := tablewriter.NewWriter(w)
        t.SetHeader(headers)
        t.SetAutoWrapText(false)
        t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
        t.SetAlignment(tablewriter.ALIGN_LEFT)
        t.SetCenterSeparator("")
        t.SetColumnSeparator("")
        t.SetRowSeparator("")
        t.SetHeaderLine(false)
        t.SetBorder(false)
        t.SetTablePadding("\t")
        t.SetNoWhiteSpace(true)
        return t
}

// healthColored returns the health string with ANSI color codes.
func healthColored(health string) string {
        switch strings.ToUpper(health) {
        case "HEALTHY":
                return color.GreenString(health)
        case "DEGRADED":
                return color.YellowString(health)
        case "UNHEALTHY":
                return color.RedString(health)
        case "UNKNOWN":
                return color.HiBlackString(health)
        default:
                return health
        }
}

// stateColored returns the lifecycle state with ANSI color codes.
func stateColored(state string) string {
        switch strings.ToUpper(state) {
        case "ACTIVE":
                return color.GreenString(state)
        case "PAUSED", "DRAINING", "RECOVERING":
                return color.YellowString(state)
        case "ERROR", "DELETED":
                return color.RedString(state)
        case "ARCHIVED":
                return color.HiBlackString(state)
        case "CREATING":
                return color.CyanString(state)
        default:
                return state
        }
}

func (p *PrettyPrinter) PrintSystemList(systems interface{}) error {
        list, ok := systems.([]model.SystemSummary)
        if !ok {
                return fmt.Errorf("expected []SystemSummary, got %T", systems)
        }
        if len(list) == 0 {
                fmt.Fprintln(p.w, "No systems found.")
                return nil
        }
        t := newTable(p.w, []string{"NAME", "NAMESPACE", "NODES", "STATE", "HEALTH", "CONN"})
        for _, sys := range list {
                t.Append([]string{
                        sys.Name,
                        sys.Namespace,
                        fmt.Sprintf("%d", sys.NodeCount),
                        stateColored(sys.State),
                        healthColored(sys.Health),
                        fmt.Sprintf("%d", sys.ConnectionCount),
                })
        }
        t.Render()
        return nil
}

func (p *PrettyPrinter) PrintSystemDetail(system interface{}) error {
        td, ok := system.(*model.SystemDetail)
        if !ok {
                return fmt.Errorf("expected *SystemDetail, got %T", system)
        }
        fmt.Fprintf(p.w, "System: %s/%s\n", td.Namespace, td.Name)
        fmt.Fprintf(p.w, "State:    %s\n", stateColored(td.State))
        fmt.Fprintf(p.w, "Health:   %s\n", healthColored(td.Health))
        fmt.Fprintf(p.w, "Version:  %d\n", td.Version)
        fmt.Fprintf(p.w, "Created:  %s\n", humanTime(td.CreatedAt))
        fmt.Fprintf(p.w, "Updated:  %s\n", humanTime(td.UpdatedAt))
        fmt.Fprintln(p.w)
        fmt.Fprintf(p.w, "Nodes (%d):\n", len(td.Nodes))
        rt := newTable(p.w, []string{"NAME", "URI", "CATEGORY", "STATE", "HEALTH"})
        for _, r := range td.Nodes {
                rt.Append([]string{r.Name, r.URI, r.Category, stateColored(r.State), healthColored(r.Health)})
        }
        rt.Render()
        return nil
}

func (p *PrettyPrinter) PrintNodeList(nodes interface{}) error {
        list, ok := nodes.([]model.NodeSummary)
        if !ok {
                return fmt.Errorf("expected []NodeSummary, got %T", nodes)
        }
        if len(list) == 0 {
                fmt.Fprintln(p.w, "No nodes found.")
                return nil
        }
        t := newTable(p.w, []string{"NAME", "system", "NAMESPACE", "URI", "STATE", "HEALTH"})
        for _, r := range list {
                t.Append([]string{r.Name, r.SystemName, r.Namespace, r.URI, stateColored(r.State), healthColored(r.Health)})
        }
        t.Render()
        return nil
}

func (p *PrettyPrinter) PrintNodeDetail(node interface{}) error {
        rd, ok := node.(*model.NodeDetail)
        if !ok {
                return fmt.Errorf("expected *NodeDetail, got %T", node)
        }
        fmt.Fprintf(p.w, "Node: %s/%s/%s\n", rd.Namespace, rd.SystemName, rd.Name)
        fmt.Fprintf(p.w, "URI:      %s\n", rd.URI)
        fmt.Fprintf(p.w, "Category: %s\n", rd.Category)
        fmt.Fprintf(p.w, "State:    %s\n", stateColored(rd.State))
        fmt.Fprintf(p.w, "Health:   %s\n", healthColored(rd.Health))
        fmt.Fprintf(p.w, "Version:  %d\n", rd.Version)
        if rd.ErrorMessage != "" {
                fmt.Fprintf(p.w, "Error:    %s\n", color.RedString(rd.ErrorMessage))
        }
        fmt.Fprintf(p.w, "Created:  %s\n", humanTime(rd.CreatedAt))
        fmt.Fprintf(p.w, "Updated:  %s\n", humanTime(rd.UpdatedAt))
        if len(rd.Config) > 0 {
                fmt.Fprintln(p.w)
                fmt.Fprintln(p.w, "Config:")
                for k, v := range rd.Config {
                        fmt.Fprintf(p.w, "  %s: %v\n", k, v)
                }
        }
        if len(rd.Labels) > 0 {
                fmt.Fprintln(p.w)
                fmt.Fprintln(p.w, "Labels:")
                for k, v := range rd.Labels {
                        fmt.Fprintf(p.w, "  %s: %s\n", k, v)
                }
        }
        printList := func(title string, items []string) {
                if len(items) == 0 {
                        return
                }
                fmt.Fprintln(p.w)
                fmt.Fprintf(p.w, "%s (%d):\n", title, len(items))
                for _, it := range items {
                        fmt.Fprintf(p.w, "  %s\n", it)
                }
        }
        printList("Listens", rd.Listens)
        printList("Events", rd.Events)
        return nil
}

func (p *PrettyPrinter) PrintNamespaceList(namespaces interface{}) error {
        list, ok := namespaces.([]model.NamespaceSummary)
        if !ok {
                return fmt.Errorf("expected []NamespaceSummary, got %T", namespaces)
        }
        if len(list) == 0 {
                fmt.Fprintln(p.w, "No namespaces found.")
                return nil
        }
        t := newTable(p.w, []string{"NAMESPACE", "SYSTEMS", "NODES", "HEALTHY", "UNHEALTHY"})
        for _, ns := range list {
                t.Append([]string{
                        ns.Namespace,
                        fmt.Sprintf("%d", ns.SystemCount),
                        fmt.Sprintf("%d", ns.NodeCount),
                        fmt.Sprintf("%d", ns.HealthyNodes),
                        fmt.Sprintf("%d", ns.UnhealthyNodes),
                })
        }
        t.Render()
        return nil
}

func (p *PrettyPrinter) PrintNamespaceDetail(detail interface{}) error {
        d, ok := detail.(*model.NamespaceDetail)
        if !ok {
                return fmt.Errorf("expected *NamespaceDetail, got %T", detail)
        }
        fmt.Fprintf(p.w, "Namespace: %s\n", d.Namespace)
        fmt.Fprintf(p.w, "Systems:   %d\n", d.SystemCount)
        fmt.Fprintf(p.w, "Nodes:     %d (healthy=%d, unhealthy=%d)\n", d.NodeCount, d.HealthyNodes, d.UnhealthyNodes)
        fmt.Fprintln(p.w)
        fmt.Fprintln(p.w, "Metrics:")
        fmt.Fprintf(p.w, "  CPU (ns):     %.1f%%\n", d.Metrics.CPU.NamespacePercent)
        fmt.Fprintf(p.w, "  Processors:   %d\n", d.Metrics.CPU.AvailableProcessors)
        fmt.Fprintf(p.w, "  Msg/s in:     %.1f\n", d.Metrics.Throughput.MessagesReceivedPerSec)
        fmt.Fprintf(p.w, "  Msg/s out:    %.1f\n", d.Metrics.Throughput.MessagesPublishedPerSec)
        fmt.Fprintf(p.w, "  Errors/s:     %.2f\n", d.Metrics.Throughput.ErrorsPerSec)
        fmt.Fprintf(p.w, "  Total in:     %d\n", d.Metrics.Throughput.TotalReceived)
        fmt.Fprintf(p.w, "  Total out:    %d\n", d.Metrics.Throughput.TotalPublished)
        fmt.Fprintf(p.w, "  Total errors: %d\n", d.Metrics.Throughput.TotalErrors)
        fmt.Fprintf(p.w, "  Heap used:    %s / %s\n", humanBytes(d.Metrics.Memory.HeapUsed), humanBytes(d.Metrics.Memory.HeapMax))
        fmt.Fprintf(p.w, "  Heap commit:  %s\n", humanBytes(d.Metrics.Memory.HeapCommitted))
        fmt.Fprintf(p.w, "  Non-heap:     %s\n", humanBytes(d.Metrics.Memory.NonHeapUsed))
        if len(d.Systems) > 0 {
                fmt.Fprintln(p.w)
                fmt.Fprintf(p.w, "Systems (%d):\n", len(d.Systems))
                t := newTable(p.w, []string{"NAME", "NODES", "STATE", "HEALTH"})
                for _, s := range d.Systems {
                        t.Append([]string{s.Name, fmt.Sprintf("%d", s.NodeCount), s.State, s.Health})
                }
                t.Render()
        }
        return nil
}

func humanBytes(b int64) string {
        if b < 1024 {
                return fmt.Sprintf("%d B", b)
        }
        if b < 1024*1024 {
                return fmt.Sprintf("%.1f KB", float64(b)/1024)
        }
        if b < 1024*1024*1024 {
                return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
        }
        return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}

func (p *PrettyPrinter) PrintRegistryList(entries interface{}) error {
        list, ok := entries.([]model.RegistryEntry)
        if !ok {
                return fmt.Errorf("expected []RegistryEntry, got %T", entries)
        }
        if len(list) == 0 {
                fmt.Fprintln(p.w, "No registry entries found.")
                return nil
        }
        t := newTable(p.w, []string{"URI", "CATEGORY", "ACTIVE", "DESCRIPTION"})
        for _, e := range list {
                active := "no"
                if e.Active {
                        active = color.GreenString("yes")
                }
                t.Append([]string{e.URI, e.Category, active, truncateDescription(e.Description, 60)})
        }
        t.Render()
        return nil
}

func (p *PrettyPrinter) PrintRegistryEntry(entry interface{}) error {
        e, ok := entry.(*model.RegistryEntry)
        if !ok {
                return fmt.Errorf("expected *RegistryEntry, got %T", entry)
        }
        fmt.Fprintf(p.w, "URI:         %s\n", e.URI)
        fmt.Fprintf(p.w, "Category:    %s\n", e.Category)
        fmt.Fprintf(p.w, "Active:      %t\n", e.Active)
        fmt.Fprintf(p.w, "Description: %s\n", e.Description)
        return nil
}

func (p *PrettyPrinter) PrintEventList(events interface{}) error {
        list, ok := events.([]model.Event)
        if !ok {
                return fmt.Errorf("expected []Event, got %T", events)
        }
        if len(list) == 0 {
                fmt.Fprintln(p.w, "No events found.")
                return nil
        }
        t := newTable(p.w, []string{"TIMESTAMP", "KIND", "node", "system", "NAMESPACE"})
        for _, e := range list {
                t.Append([]string{humanTime(e.Timestamp), e.Kind, e.NodeName, e.SystemName, e.Namespace})
        }
        t.Render()
        return nil
}

func (p *PrettyPrinter) PrintHealthSummary(health interface{}) error {
        h, ok := health.(*model.HealthSummary)
        if !ok {
                return fmt.Errorf("expected *HealthSummary, got %T", health)
        }
        fmt.Fprintf(p.w, "Overall:    %s\n", healthColored(h.Overall))
        fmt.Fprintf(p.w, "systems: %d\n", h.TotalSystems)
        fmt.Fprintf(p.w, "Nodes:  %d total (%s, %s, %s, %s)\n",
                h.TotalNodes,
                color.GreenString("%d healthy", h.HealthyNodes),
                color.YellowString("%d degraded", h.DegradedNodes),
                color.RedString("%d unhealthy", h.UnhealthyNodes),
                color.HiBlackString("%d unknown", h.UnknownNodes),
        )
        return nil
}

func (p *PrettyPrinter) PrintSystemHealth(health interface{}) error {
        h, ok := health.(*model.SystemHealth)
        if !ok {
                return fmt.Errorf("expected *SystemHealth, got %T", health)
        }
        fmt.Fprintf(p.w, "System:  %s/%s\n", h.Namespace, h.SystemName)
        fmt.Fprintf(p.w, "Overall:   %s\n", healthColored(h.Overall))
        fmt.Fprintf(p.w, "Nodes: %d\n", h.NodeCount)
        if len(h.PerNode) > 0 {
                fmt.Fprintln(p.w)
                t := newTable(p.w, []string{"node", "HEALTH"})
                for name, status := range h.PerNode {
                        t.Append([]string{name, healthColored(status)})
                }
                t.Render()
        }
        return nil
}

func (p *PrettyPrinter) PrintNodeHealth(health interface{}) error {
        h, ok := health.(*model.NodeHealth)
        if !ok {
                return fmt.Errorf("expected *NodeHealth, got %T", health)
        }
        fmt.Fprintf(p.w, "Node: %s\n", h.NodeName)
        fmt.Fprintf(p.w, "State:    %s\n", stateColored(h.State))
        fmt.Fprintf(p.w, "Health:   %s\n", healthColored(h.Health))
        fmt.Fprintf(p.w, "Version:  %d\n", h.Version)
        if h.ErrorMessage != "" {
                fmt.Fprintf(p.w, "Error:    %s\n", color.RedString(h.ErrorMessage))
        }
        if len(h.RecentEvents) > 0 {
                fmt.Fprintln(p.w)
                fmt.Fprintf(p.w, "Recent Events (%d):\n", len(h.RecentEvents))
                t := newTable(p.w, []string{"TIMESTAMP", "KIND", "node"})
                for _, e := range h.RecentEvents {
                        t.Append([]string{humanTime(e.Timestamp), e.Kind, e.NodeName})
                }
                t.Render()
        }
        return nil
}

func (p *PrettyPrinter) PrintDeployResult(result interface{}) error {
        switch v := result.(type) {
        case *model.DeploySystemResponse:
                fmt.Fprintf(p.w, "%s System %s/%s deployed.\n", color.GreenString("✓"), v.Namespace, v.Name)
                fmt.Fprintf(p.w, "  Nodes:  %d\n", v.NodeCount)
                fmt.Fprintf(p.w, "  State:  %s\n", stateColored(v.State))
                fmt.Fprintf(p.w, "  Health: %s\n", healthColored(v.Health))
                if len(v.Nodes) > 0 {
                        fmt.Fprintln(p.w)
                        fmt.Fprintf(p.w, "  Nodes (%d):\n", len(v.Nodes))
                        for _, n := range v.Nodes {
                                fmt.Fprintf(p.w, "    - %s\n", n)
                        }
                }
                return nil
        case *DeployFailurePayload:
                fmt.Fprintf(p.w, "%s Deploy failed: %s\n", color.RedString("✗"), v.Message)
                if len(v.Errors) > 0 {
                        fmt.Fprintln(p.w)
                        fmt.Fprintf(p.w, "Validation errors (%d):\n", len(v.Errors))
                        for _, e := range v.Errors {
                                fmt.Fprintf(p.w, "  %s  %s: %s\n", color.RedString(e.Severity), e.Path, e.Message)
                        }
                }
                return nil
        }
        return fmt.Errorf("unknown deploy result type %T", result)
}

// DeployFailurePayload is a wrapper for pretty-printing deploy failures.
type DeployFailurePayload struct {
        Message string
        Errors  []model.ValidationError
}

func (p *PrettyPrinter) PrintRaw(value interface{}) error {
        fmt.Fprintf(p.w, "%v\n", value)
        return nil
}

func (p *PrettyPrinter) PrintSuccess(message string) error {
        fmt.Fprintf(p.w, "%s %s\n", color.GreenString("✓"), message)
        return nil
}

func (p *PrettyPrinter) PrintError(err error) error {
        fmt.Fprintf(os.Stderr, "%s %s\n", color.RedString("✗"), err.Error())
        return nil
}

// humanTime formats a time.Time for human display.
func humanTime(t time.Time) string {
        if t.IsZero() {
                return "-"
        }
        // If within 24h, show relative; else show date.
        diff := time.Since(t)
        if diff < 24*time.Hour && diff > -24*time.Hour {
                return fmt.Sprintf("%s ago", durationHuman(diff))
        }
        return t.Format("2006-01-02 15:04:05")
}

func durationHuman(d time.Duration) string {
        if d < time.Minute {
                return fmt.Sprintf("%ds", int(d.Seconds()))
        }
        if d < time.Hour {
                return fmt.Sprintf("%dm", int(d.Minutes()))
        }
        return fmt.Sprintf("%dh", int(d.Hours()))
}

// truncateDescription trims a string to maxLen, appending "..." if truncated.
func truncateDescription(s string, maxLen int) string {
        if len(s) <= maxLen {
                return s
        }
        return s[:maxLen-3] + "..."
}
