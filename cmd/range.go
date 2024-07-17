package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/fatih/color"
	client "github.com/jamf/regatta-go"
	"github.com/spf13/cobra"
)

var (
	rangeBinary     bool
	rangeLimit      int64
	rangeOutput     = plainFormat
	rangeValuesOnly bool

	zero = string([]byte{0})
)

func init() {
	RangeCmd.Flags().BoolVar(&rangeBinary, "binary", false, "avoid decoding keys and values into UTF-8 strings, but rather encode them as Base64 strings")
	RangeCmd.Flags().Int64Var(&rangeLimit, "limit", 0, "limit number of returned items. Zero is no limit")
	RangeCmd.Flags().Var(&rangeOutput, "output", "configure output format. Currently plain, json and jsonl is supported")
	RangeCmd.RegisterFlagCompletionFunc("output", outputFormatCompletion)
	RangeCmd.Flags().BoolVar(&rangeValuesOnly, "values-only", false, "return only values")
}

// RangeCmd is a subcommand used for retrieving records from a table.
var RangeCmd = cobra.Command{
	Use:   "range <table> [key/prefix*] [range_end]",
	Short: "Retrieve data from Regatta store",
	Long: "Retrieves data from Regatta store using Range query as defined in API (https://engineering.jamf.com/regatta/api/#range).\n" +
		"You can either retrieve all items from the Regatta by providing no key.\n" +
		"Or you can query for a single item in Regatta by providing item's key.\n" +
		"Or you can query for all items with given prefix, by providing the given prefix and adding the asterisk (*) to the prefix.\n" +
		"Or you can query for all items in given range, by providing lexicographic range from start (inclusive) to end (exclusive) key\n" +
		"When key or prefix is provided, it needs to be valid UTF-8 string.",
	Example: "regatta-client range table\n" +
		"regatta-client range table key\n" +
		"regatta-client range table 'prefix*'\n" +
		"regatta-client range table key range_end",
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.MaximumNArgs(3)),
	ValidArgsFunction: func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		var comps []string
		if len(args) == 0 {
			comps = cobra.AppendActiveHelp(comps, "You must specify a Regatta table name")
		} else if len(args) == 1 {
			comps = cobra.AppendActiveHelp(comps, "You can provide a key or prefix to search for. If not provided all items from the table is returned.")
		} else if len(args) == 2 {
			comps = cobra.AppendActiveHelp(comps, "You can provide range_end, which has to be lexicographically greater than provided key")
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	},
	PreRunE: connect,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		key, opts := keyAndOptsForRange(args)

		var resps []*client.GetResponse
		bufferAll := rangeOutput == jsonFormat
		iterator, err := regatta.Table(args[0]).Iterate(ctx, key, opts...)
		if err != nil {
			handleRegattaError(cmd, err)
			return
		}

		iterator(func(resp *client.GetResponse, err error) bool {
			if err != nil {
				handleRegattaError(cmd, err)
				return false
			}
			if bufferAll {
				// collect all responses and print when we have everything
				resps = append(resps, resp)
			} else {
				// print while paging
				switch rangeOutput {
				case plainFormat:
					plainPrint(cmd, resp)
				case jsonLineFormat:
					jsonLinePrint(cmd, resp)
				}
			}
			return true
		})

		if bufferAll {
			switch rangeOutput {
			case jsonFormat:
				jsonPrint(cmd, resps)
			}
		}
	},
}

type rangeCommandResult struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value"`
}

func jsonPrint(cmd *cobra.Command, resps []*client.GetResponse) {
	results := make([]rangeCommandResult, 0)
	for _, resp := range resps {
		for _, kv := range resp.Kvs {
			key := getValue(kv.Key)
			if rangeValuesOnly {
				key = ""
			}
			results = append(results, rangeCommandResult{Key: key, Value: getValue(kv.Value)})
		}
	}

	marshal, _ := json.Marshal(results)
	cmd.Println(string(marshal))
}

func plainPrint(cmd *cobra.Command, resp *client.GetResponse) {
	for _, kv := range resp.Kvs {
		key := color.New(color.FgBlue).Sprint(getValue(kv.Key))
		value := color.New(color.FgGreen).Sprint(getValue(kv.Value))
		if rangeValuesOnly {
			cmd.Println(value)
		} else {
			cmd.Println(key + ": " + value)
		}
	}
}

func jsonLinePrint(cmd *cobra.Command, resp *client.GetResponse) {
	for _, kv := range resp.Kvs {
		key := getValue(kv.Key)
		if rangeValuesOnly {
			key = ""
		}
		res := rangeCommandResult{Key: key, Value: getValue(kv.Value)}
		marshal, _ := json.Marshal(res)
		cmd.Println(string(marshal))
	}
}

func keyAndOptsForRange(args []string) (string, []client.OpOption) {
	if len(args) == 3 {
		start := args[1]
		end := args[2]
		return start, []client.OpOption{client.WithRange(end), client.WithLimit(rangeLimit)}
	}
	if len(args) == 2 {
		key := args[1]
		if strings.HasSuffix(key, "*") {
			key = strings.TrimSuffix(key, "*")
			if len(key) == 0 {
				// get all
				return zero, []client.OpOption{client.WithRange(zero), client.WithLimit(rangeLimit)}
			}
			// prefix search
			return key, []client.OpOption{client.WithPrefix(), client.WithLimit(rangeLimit)}
		}
		return key, []client.OpOption{}
	}
	// get all
	return zero, []client.OpOption{client.WithRange(zero), client.WithLimit(rangeLimit)}
}

func getValue(data []byte) string {
	if rangeBinary {
		return base64.StdEncoding.EncodeToString(data)
	}
	return string(data)
}
