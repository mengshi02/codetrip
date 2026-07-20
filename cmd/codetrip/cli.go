package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip"
	"github.com/spf13/cobra"
)

type cliFlags struct {
	tripDir string
	verbose bool
}

func newCLIFlags() *cliFlags { return &cliFlags{} }

func (f *cliFlags) resolvedTripDir() (string, error) {
	if f.tripDir != "" {
		return f.tripDir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codetrip"), nil
}

func (f *cliFlags) openTrip() (*codetrip.Engine, error) {
	dir, err := f.resolvedTripDir()
	if err != nil {
		return nil, err
	}
	return codetrip.Open(dir)
}

func newRootCmd(flags *cliFlags) *cobra.Command {
	root := &cobra.Command{
		Use:   "codetrip",
		Short: "Production code graph ingestion and retrieval engine",
	}
	root.PersistentFlags().StringVar(&flags.tripDir, "dir", "", "data directory (default: ~/.codetrip)")
	root.PersistentFlags().BoolVarP(&flags.verbose, "verbose", "v", false, "enable info logging")
	root.AddCommand(newIndexCmd(flags), newSearchCmd(flags), newSourceCmd(flags), newEmbedCmd(flags), newHybridCmd(flags), newTraverseCmd(flags), newPathCmd(flags), newExportCmd(flags), newListCmd(flags), newMCPCmd(flags), newVersionCmd())
	return root
}

func newSourceCmd(flags *cliFlags) *cobra.Command {
	var repo string
	var limit, contextLines int
	command := &cobra.Command{
		Use: "source <query>", Short: "Search file names and source contents", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			result, err := trip.SearchSource(cmd.Context(), &codetrip.SourceSearchRequest{Repo: repo, Query: args[0], Limit: limit, ContextLines: contextLines})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().IntVar(&limit, "limit", 20, "maximum matches")
	command.Flags().IntVar(&contextLines, "context", 1, "context lines before and after each match")
	_ = command.MarkFlagRequired("repo")
	return command
}

type embeddingFlags struct {
	endpoint, model, apiKey string
	dimensions              int
}

func (f *embeddingFlags) add(command *cobra.Command) {
	command.Flags().StringVar(&f.endpoint, "endpoint", "", "OpenAI-compatible embeddings endpoint")
	command.Flags().StringVar(&f.model, "model", "", "embedding model name")
	command.Flags().StringVar(&f.apiKey, "api-key", "", "embedding API key (or CODETRIP_EMBEDDING_API_KEY)")
	command.Flags().IntVar(&f.dimensions, "dimensions", 384, "embedding vector dimensions")
	_ = command.MarkFlagRequired("endpoint")
}

func (f *embeddingFlags) embedder() *codetrip.HTTPEmbedder {
	key := f.apiKey
	if key == "" {
		key = os.Getenv("CODETRIP_EMBEDDING_API_KEY")
	}
	return codetrip.NewHTTPEmbedder(f.endpoint, f.model, key, f.dimensions)
}

func newEmbedCmd(flags *cliFlags) *cobra.Command {
	var repo string
	var quantize bool
	embeddingConfig := &embeddingFlags{}
	command := &cobra.Command{
		Use: "embed", Short: "Build semantic vectors for an indexed repository", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			result, err := trip.EmbedRepo(cmd.Context(), repo, embeddingConfig.embedder(), &codetrip.EmbedOptions{QuantizeInt8: quantize})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().BoolVar(&quantize, "quantize-int8", false, "build an int8 vector file")
	embeddingConfig.add(command)
	_ = command.MarkFlagRequired("repo")
	return command
}

func newHybridCmd(flags *cliFlags) *cobra.Command {
	var repo string
	var limit int
	embeddingConfig := &embeddingFlags{}
	command := &cobra.Command{
		Use: "hybrid <query>", Short: "Fuse lexical and semantic vector retrieval", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			if err := trip.AttachEmbedder(repo, embeddingConfig.embedder()); err != nil {
				return err
			}
			result, err := trip.HybridSearch(cmd.Context(), &codetrip.HybridSearchRequest{Repo: repo, Query: args[0], Limit: limit})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().IntVar(&limit, "limit", 20, "maximum results")
	embeddingConfig.add(command)
	_ = command.MarkFlagRequired("repo")
	return command
}

func newPathCmd(flags *cliFlags) *cobra.Command {
	var repo string
	command := &cobra.Command{
		Use: "path <source-node-id> <target-node-id>", Short: "Find the shortest directed path between two graph nodes", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			result, err := trip.ShortestPath(cmd.Context(), &codetrip.PathRequest{
				Repo: repo, SourceNodeID: args[0], TargetNodeID: args[1],
			})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	_ = command.MarkFlagRequired("repo")
	return command
}

func newTraverseCmd(flags *cliFlags) *cobra.Command {
	var repo, direction, relations string
	var depth int
	command := &cobra.Command{
		Use: "traverse <node-id>", Short: "Traverse the code graph with bounded BFS", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			var relationTypes []string
			if relations != "" {
				relationTypes = strings.Split(relations, ",")
			}
			result, err := trip.Traverse(cmd.Context(), &codetrip.TraverseRequest{
				Repo: repo, StartNodeID: args[0], Direction: codetrip.TraverseDirection(direction),
				MaxDepth: depth, RelationTypes: relationTypes,
			})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().StringVar(&direction, "direction", "out", "edge direction: out, in, or both")
	command.Flags().IntVar(&depth, "depth", 3, "maximum traversal depth")
	command.Flags().StringVar(&relations, "relations", "", "comma-separated relationship types")
	_ = command.MarkFlagRequired("repo")
	return command
}

func newSearchCmd(flags *cliFlags) *cobra.Command {
	var repo string
	var limit int
	command := &cobra.Command{
		Use: "search <query>", Short: "Search indexed code symbols", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			result, err := trip.Search(cmd.Context(), &codetrip.SearchRequest{Repo: repo, Query: args[0], Limit: limit})
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().IntVar(&limit, "limit", 20, "maximum results")
	_ = command.MarkFlagRequired("repo")
	return command
}

func newExportCmd(flags *cliFlags) *cobra.Command {
	var repo string
	var output string
	command := &cobra.Command{
		Use: "export", Short: "Export the complete persisted graph as CSV", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			manifest, err := trip.ExportFullCSV(repo, output)
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(manifest, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repo, "repo", "", "repository name")
	command.Flags().StringVar(&output, "output", "", "output directory")
	_ = command.MarkFlagRequired("repo")
	_ = command.MarkFlagRequired("output")
	return command
}

func newIndexCmd(flags *cliFlags) *cobra.Command {
	var repoName string
	var csvPath string
	var strict bool
	var replace bool
	command := &cobra.Command{
		Use:   "index <repository>",
		Short: "Parse a repository and persist its code graph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			options := make([]codetrip.IndexOption, 0, 3)
			if repoName != "" {
				options = append(options, codetrip.WithRepoName(repoName))
			}
			if csvPath != "" {
				options = append(options, codetrip.WithCSVExport(csvPath), codetrip.WithCSVExportStrict(strict))
			}
			if replace {
				options = append(options, codetrip.WithReplaceExisting(true))
			}
			result, err := trip.IndexRepo(cmd.Context(), args[0], options...)
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
	command.Flags().StringVar(&repoName, "repo", "", "repository name")
	command.Flags().StringVar(&csvPath, "export", "", "write validation CSV files to this directory")
	command.Flags().BoolVar(&strict, "export-strict", false, "fail indexing when CSV export fails")
	command.Flags().BoolVar(&replace, "replace", false, "atomically replace an existing repository snapshot")
	return command
}

func newListCmd(flags *cliFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List indexed repositories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()
			repositories, err := trip.ListRepos()
			if err != nil {
				return err
			}
			encoded, _ := json.MarshalIndent(repositories, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(encoded))
			return nil
		},
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use: "version", Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) { fmt.Fprintln(cmd.OutOrStdout(), codetrip.Version) },
	}
}
