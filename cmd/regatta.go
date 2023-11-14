package cmd

import (
	client "github.com/jamf/regatta-go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func connect(cmd *cobra.Command, _ []string) error {
	// Allow for mocking in tests.
	if regatta == nil {
		cl, err := client.New(
			&client.ConfigSpec{Endpoints: []string{endpointOption}, DialTimeout: dialTimeout},
			client.WithSecureConfig(&client.SecureConfig{Cacert: certOption, InsecureSkipVerify: insecureOption}),
			client.WithDialOptions(grpc.WithBlock(), grpc.WithReturnConnectionError()),
		)
		if err != nil {
			cmd.PrintErrln("There was an error, while establishing connection to Regatta.")
			return err
		}
		regatta = cl
	}
	return nil
}

func handleRegattaError(cmd *cobra.Command, err error) {
	if st := status.Convert(err); st != nil {
		switch st.Code() {
		case codes.NotFound:
			cmd.PrintErrln("The requested resource was not found:", st.Message())
		case codes.Unavailable:
			cmd.PrintErrln("Regatta is not reachable:", st.Message())
		default:
			cmd.PrintErrf("Received RPC error from Regatta, code '%s' with message '%s'\n", st.Code(), st.Message())
		}
	} else {
		cmd.PrintErrln("There was an error, while querying Regatta.", err)
	}
}
