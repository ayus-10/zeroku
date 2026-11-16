package proxy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ayus-10/zeroku/internal/app"
	"github.com/ayus-10/zeroku/internal/state"
)

const stagingCA = "https://acme-staging-v02.api.letsencrypt.org/directory"

type App struct {
	Name          string
	ContainerPort int
}

type Options struct {
	BaseDomain  string
	ControlAddr string
	ACMEStaging bool
}

func Render(o Options, apps []state.App) string {
	var b strings.Builder

	if o.ACMEStaging {
		fmt.Fprintf(&b, "{\n\tacme_ca %s\n}\n\n", stagingCA)
	}

	fmt.Fprintf(&b, "%s.%s {\n\treverse_proxy %s {\n\t\tflush_interval -1\n\t}\n}\n\n",
		app.ControlPlaneName, o.BaseDomain, o.ControlAddr)

	sorted := append([]state.App(nil), apps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, a := range sorted {
		if a.ContainerPort == 0 || app.ValidateRouteName(a.Name) != nil {
			continue
		}
		fmt.Fprintf(&b, "%s.%s {\n\treverse_proxy 127.0.0.1:%d\n}\n\n", a.Name, o.BaseDomain, a.ContainerPort)
	}
	return b.String()
}
