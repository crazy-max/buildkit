package debug

import (
	"fmt"
	"os"
	"text/tabwriter"

	bccommon "github.com/moby/buildkit/cmd/buildctl/common"
	"github.com/urfave/cli"
)

var InfoCommand = cli.Command{
	Name:   "info",
	Usage:  "display internal information",
	Action: info,
}

const defaultPfx = "  "

func info(clicontext *cli.Context) error {
	c, err := bccommon.ResolveClient(clicontext)
	if err != nil {
		return err
	}
	res, err := c.Info(bccommon.CommandContext(clicontext))
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	_, _ = fmt.Fprintf(w, "BuildKit:\t\n")
	_, _ = fmt.Fprintf(w, "%sPackage:\t%s\n", defaultPfx, res.BuildkitPackage)
	_, _ = fmt.Fprintf(w, "%sVersion:\t%s\n", defaultPfx, res.BuildkitVersion)
	_, _ = fmt.Fprintf(w, "%sRevision:\t%s\n", defaultPfx, res.BuildkitRevision)
	_ = w.Flush()

	_, _ = fmt.Fprintf(w, "\t\n")
	_, _ = fmt.Fprintf(w, "Built-in Dockerfile Frontend:\t\n")
	_, _ = fmt.Fprintf(w, "%sVersion:\t%s\n", defaultPfx, res.DockerfileFrontendVersion)

	return w.Flush()
}
