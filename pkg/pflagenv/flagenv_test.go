package pflagenv_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/bitnami-labs/sealed-secrets/pkg/pflagenv"
	flag "github.com/spf13/pflag"
)

func TestPflagenv(t *testing.T) {
	testCases := []struct {
		set  bool
		val  string
		want string
	}{
		{false, "", "default"},
		{true, "bar", "bar"},
		{true, "", ""},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			defer os.Unsetenv("MY_TEST_FOO")

			if tc.set {
				os.Setenv("MY_TEST_FOO", tc.val)
			}

			fs := flag.NewFlagSet("test", flag.PanicOnError)
			s := fs.String("foo", "default", "help")
			pflagenv.SetFlagsFromEnv("MY_TEST", fs)

			_ = fs.Parse(nil)

			if got, want := *s, tc.want; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}
