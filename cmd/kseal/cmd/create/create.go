/*
Copyright 2022 - Bitnami <containers@bitnami.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package create

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

// CreateSealedSecretOptions holds the options for "create" sub command
type CreateSealedSecretOptions struct {
	// Name of SealedSecret (required)
	Name string
	// Namespace of SealedSecret (optional)
	Namespace string
	// Type of SealedSecret (required)
	Type string
	// FileSources to derive the secret from (optional)
	FileSources []string
	// LiteralSources to derive the secret from (optional)
	LiteralSources []string
	// KeyCertificate used to sign the SealedSecret (optional)
	KeyCertificate string
	// ClusterWide SealedSecret (optional)
	ClusterWide bool
	// Output (required)
	Output string
}

// NewCmdCreate creates a command object for the "create" action.
func NewCmdCreate() *cobra.Command {
	o := CreateSealedSecretOptions{}

	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Sealed Secret",
		Long: `Create a Sealed Secret using a public key.

Examples:

    kseal create sealed-secret-name --namespace myns --from-literal=item1=value1 --from-literal=item2=value2
	kseal create sealed-secret-name --namespace myns --from-file=item1=file1.txt
	kseal create sealed-secret-name --namespace myns --from-literal=item1=value1 --scope cluster-wide
	kseal create sealed-secret-name --namespace myns --from-literal=item1=value1 --type secret-type
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]
			if err := o.Validate(); err != nil {
				return err
			}
			// TODO
			return nil
		},
	}

	// Flags
	createCmd.Flags().StringVar(&o.KeyCertificate, "cert", "", "Certificate / public key file/URL to use for encryption")
	createCmd.Flags().StringSliceVar(&o.FileSources, "from-file", o.FileSources, "Key files can be specified using their file path, in which case a default name will be given to them, or optionally with a name and file path, in which case the given name will be used.")
	createCmd.Flags().StringArrayVar(&o.LiteralSources, "from-literal", o.LiteralSources, "Specify a key and literal value to insert in sealed secret (i.e. mykey=somevalue)")
	createCmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Namespace of SealedSecret object to create")
	createCmd.Flags().StringVarP(&o.Output, "output", "o", "json", "Output format. Supported values are: json, yaml")
	createCmd.Flags().StringVar(&o.Type, "type", "generic", "SealedSecret type")
	createCmd.Flags().BoolVar(&o.ClusterWide, "cluster-wide", false, "Use cluster-wide scope")

	return createCmd
}

// Validate checks if CreateSealedSecretOptions has sufficient value to run
func (o *CreateSealedSecretOptions) Validate() error {
	if len(o.Name) == 0 {
		return fmt.Errorf("name must be specified")
	}
	if len(o.FileSources) > 0 || len(o.LiteralSources) > 0 {
		return fmt.Errorf("from-file cannot be combined with from-literal")
	}
	if len(o.Type) == 0 {
		return fmt.Errorf("type must be specified")
	}
	if len(o.Output) == 0 {
		return fmt.Errorf("output must be specified")
	}
	if regexp.MustCompile(`json|tank`).MatchString(o.Output) {
		return fmt.Errorf("Invalid output format. Allowed values are: json, yaml")
	}
	return nil
}
