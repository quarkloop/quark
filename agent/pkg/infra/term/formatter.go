package term

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"
)

type SpaceRow struct {
	ID        string
	Name      string
	Status    string
	Port      int
	Dir       string
	PID       int
	CreatedAt time.Time
}

func PrintSpaceTable(rows []SpaceRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tPORT\tDIR")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", r.ID, r.Name, r.Status, r.Port, r.Dir)
	}
	w.Flush()
}

func PrintSpaceDetail(r SpaceRow) {
	fmt.Printf("ID:      %s\n", r.ID)
	fmt.Printf("Name:    %s\n", r.Name)
	fmt.Printf("Status:  %s\n", r.Status)
	fmt.Printf("Dir:     %s\n", r.Dir)
	fmt.Printf("Port:    %d\n", r.Port)
	fmt.Printf("PID:     %d\n", r.PID)
	fmt.Printf("Created: %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"))
}
