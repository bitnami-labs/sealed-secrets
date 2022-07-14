// Package pflagenv implements a simple way to expose all your pflag flags as environmental variables.
//
// Commandline flags have more precedence over environment variables.
// In order to use it just call pflagenv.SetFlagsFromEnv from an init function or from your main.
//
// You can call it either before or after your your flag.Parse invocation.
//
// This example will make it possible to set the default of --my_flag also via the MY_PROG_MY_FLAG
// env var:
//
//    var myflag = pflag.String("my_flag", "", "some flag")
//
//    func init() {
//        pflagenv.SetFlagsFromEnv("MY_PROG", pflag.CommandLine)
//    }
//
//    func main() {
//        pflag.Parse()
//        ...
//    }
package pflagenv

import (
	"fmt"
	"os"
	"strings"

	flag "github.com/spf13/pflag"
)

// SetFlagsFromEnv sets flag values from environment, e.g. PREFIX_FOO_BAR set -foo_bar.
// It sets only flags that haven't been set explicitly. The defaults are preserved and -help
// will still show the defaults provided in the code.
func SetFlagsFromEnv(prefix string, fs *flag.FlagSet) {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		set[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		// ignore flags set from the commandline
		if set[f.Name] {
			return
		}
		// remove trailing _ to reduce common errors with the prefix, i.e. people setting it to MY_PROG_
		cleanPrefix := strings.TrimSuffix(prefix, "_")
		name := fmt.Sprintf("%s_%s", cleanPrefix, strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
		if e, ok := os.LookupEnv(name); ok {
			_ = f.Value.Set(e)
		}
	})
}
