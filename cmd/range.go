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
	rangeBinary bool
	rangeLimit  int64
	output      = plainFormat

	zero = string([]byte{0})
)

func init() {
	Range.Flags().BoolVar(&rangeBinary, "binary", false, "avoid decoding keys and values into UTF-8 strings, but rather encode them as Base64 strings")
	Range.Flags().Int64Var(&rangeLimit, "limit", 0, "limit number of returned items. Zero is no limit")
	Range.Flags().Var(&output, "output", "configure output format. Currently plain and json is supported")
	Range.RegisterFlagCompletionFunc("output", outputFormatCompletion)
}

// Range is a subcommand used for retrieving records from a table.
var Range = cobra.Command{
	Use:   "range <table> [key]",
	Short: "Retrieve data from Regatta store",
	Long: "Retrieves data from Regatta store using Range query as defined in API (https://engineering.jamf.com/regatta/api/#range).\n" +
		"You can either retrieve all items from the Regatta by providing no key.\n" +
		"Or you can query for a single item in Regatta by providing item's key.\n" +
		"Or you can query for all items with given prefix, by providing the given prefix and adding the asterisk (*) to the prefix.\n" +
		"When key or prefix is provided, it needs to be valid UTF-8 string.",
	Example: "regatta-client range table\n" +
		"regatta-client range table key\n" +
		"regatta-client range table 'prefix*'",
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.MaximumNArgs(2)),
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var comps []string
		if len(args) == 0 {
			comps = cobra.AppendActiveHelp(comps, "You must specify a Regatta table name")
		} else if len(args) == 1 {
			comps = cobra.AppendActiveHelp(comps, "You can provide a key or prefix to search for. If not provided all items from the table is returned.")
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	},
	PreRunE: connect,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		key, opts := keyAndOptsForRange(args)

		var resps []*client.GetResponse
		var err error
		firstRegattaCall := true
		bufferAll := output == jsonFormat

		var total int64

		for resp := (*client.GetResponse)(nil); resp == nil || resp.More; {
			resp, err = regatta.Table(args[0]).Get(ctx, key, opts...)
			if err != nil {
				handleRegattaError(cmd, err)
				return
			}

			if !firstRegattaCall && len(resp.Kvs) > 0 {
				// remove the duplicated key due to paging
				resp.Kvs = resp.Kvs[1:]
				resp.Count--
			}

			if rangeLimit != 0 && total+resp.Count > rangeLimit {
				// cut above limit
				toRemove := int(total + resp.Count - rangeLimit)
				resp.Kvs = resp.Kvs[:(len(resp.Kvs) - toRemove)]
				resp.Count = int64(len(resp.Kvs))
			}

			total += resp.Count

			if bufferAll {
				resps = append(resps, resp)
			} else {
				results := make([]rangeCommandResult, 0)
				for _, kv := range resp.Kvs {
					results = append(results, rangeCommandResult{Key: getValue(kv.Key), Value: getValue(kv.Value)})
				}
				switch output {
				case plainFormat:
					plainPrint(cmd, results)
				}
			}

			if rangeLimit != 0 && total == rangeLimit {
				// we have enough data according to limit, no need to page
				break
			}
			if resp.More {
				// the same key will be included in next page
				key = string(resp.Kvs[len(resp.Kvs)-1].Key)
				firstRegattaCall = false
			}
		}

		if bufferAll {
			results := make([]rangeCommandResult, 0)
			for _, resp := range resps {
				for _, kv := range resp.Kvs {
					results = append(results, rangeCommandResult{Key: getValue(kv.Key), Value: getValue(kv.Value)})
				}
			}

			switch output {
			case jsonFormat:
				jsonPrint(cmd, results)
			}
		}
	},
}

func jsonPrint(cmd *cobra.Command, result []rangeCommandResult) {
	marshal, _ := json.Marshal(result)
	cmd.Println(string(marshal))
}

func plainPrint(cmd *cobra.Command, result []rangeCommandResult) {
	for _, v := range result {
		key := color.New(color.FgBlue).Sprint(v.Key)
		value := color.New(color.FgGreen).Sprint(v.Value)
		cmd.Println(key + ": " + value)
	}
}

type rangeCommandResult struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func keyAndOptsForRange(args []string) (string, []client.OpOption) {
	if len(args) == 2 {
		key := args[1]
		if strings.HasSuffix(key, "*") {
			key = strings.TrimSuffix(key, "*")
			if len(key) == 0 {
				// get all
				return zero, []client.OpOption{client.WithRange(zero), client.WithLimit(rangeLimit)}
			}
			// prefix search
			return key, []client.OpOption{client.WithRange(client.GetPrefixRangeEnd(key)), client.WithLimit(rangeLimit)}
		}
		// get by ID
		return key, []client.OpOption{client.WithLimit(rangeLimit)}
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
