package proxy

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ayus-10/zeroku/internal/state"
)

const stagingCA = "https://acme-staging-v02.api.letsencrypt.org/directory"
const ControlPlaneName = "zeroku"

type Options struct {
	BaseDomain  string
	ControlAddr string
	ACMEStaging bool
}

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

func ValidateAppName(name string) error {
	if name == "zeroku" {
		return fmt.Errorf("%q is reserved for the control plane", name)
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("invalid app name %q: use lowercase letters, digits and hyphens (1-63 chars, no leading/trailing hyphen)", name)
	}
	return nil
}

func Render(o Options, apps []state.App) string {
	var b strings.Builder

	if o.ACMEStaging {
		fmt.Fprintf(&b, "{\n\tacme_ca %s\n}\n\n", stagingCA)
	}

	fmt.Fprintf(&b, "%s.%s {\n\treverse_proxy %s {\n\t\tflush_interval -1\n\t}\n}\n\n",
		ControlPlaneName, o.BaseDomain, o.ControlAddr)

	sorted := append([]state.App(nil), apps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, a := range sorted {
		if a.ContainerPort == 0 || ValidateAppName(a.Name) != nil {
			continue
		}
		fmt.Fprintf(&b, "%s.%s {\n\treverse_proxy 127.0.0.1:%d\n}\n\n", a.Name, o.BaseDomain, a.ContainerPort)
	}
	return b.String()
}
