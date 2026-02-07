package cmd

// blocks.go displays token usage grouped into 5-hour windows that match
// Claude's rate limit reset cadence (ported from ccusage). This helps users
// understand their burn rate and predict when they'll hit limits.

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Pilan-AI/mnemo/internal/db"
)

var blocksActive bool
var blocksRecent int
var blocksDays int

var blocksCmd = &cobra.Command{
	Use:   "blocks",
	Short: "Show 5-hour usage blocks (Claude rate limit windows)",
	Long: `Display token usage grouped into 5-hour blocks.

Claude's rate limits reset every 5 hours. This command shows your usage
patterns within these windows, including burn rates and projections.

Examples:
  mnemo blocks              # Show all blocks from last 7 days
  mnemo blocks --active     # Show current active block with projections
  mnemo blocks --recent 3   # Show last 3 blocks
  mnemo blocks --days 14    # Show blocks from last 14 days`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.InitDB(); err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.CloseDB()

		if blocksActive {
			return showActiveBlock()
		}

		if blocksRecent > 0 {
			return showRecentBlocks(blocksRecent)
		}

		return showAllBlocks(blocksDays)
	},
}

func init() {
	blocksCmd.Flags().BoolVarP(&blocksActive, "active", "a", false, "Show only the current active block")
	blocksCmd.Flags().IntVarP(&blocksRecent, "recent", "r", 0, "Show last N blocks")
	blocksCmd.Flags().IntVarP(&blocksDays, "days", "d", 7, "Days of history to analyze")
	rootCmd.AddCommand(blocksCmd)
}

func showActiveBlock() error {
	block, err := db.GetActiveBlock()
	if err != nil {
		return fmt.Errorf("failed to get active block: %w", err)
	}

	if block == nil {
		fmt.Println("No active block. Start a coding session to begin tracking!")
		return nil
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("                    CURRENT 5-HOUR BLOCK                      ")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	remaining := block.RemainingTime()
	hours := int(remaining.Hours())
	mins := int(remaining.Minutes()) % 60

	fmt.Printf("  Window:    %s → %s\n",
		block.StartTime.Format("15:04"),
		block.EndTime.Format("15:04"))
	fmt.Printf("  Remaining: %dh %dm\n", hours, mins)
	fmt.Println()

	fmt.Println("  ┌─────────────────┬────────────────┬────────────────┐")
	fmt.Println("  │     Metric      │    Current     │   Projected    │")
	fmt.Println("  ├─────────────────┼────────────────┼────────────────┤")
	fmt.Printf("  │ Tokens          │ %14s │ %14s │\n",
		formatNumber(block.TotalTokens()),
		formatNumber(block.ProjectedTokens()))
	fmt.Printf("  │ Cost            │ %14s │ %14s │\n",
		fmt.Sprintf("$%.4f", block.CostUSD),
		fmt.Sprintf("$%.4f", block.ProjectedCost()))
	fmt.Println("  └─────────────────┴────────────────┴────────────────┘")
	fmt.Println()

	fmt.Printf("  Burn Rate: %.0f tokens/min • $%.4f/hour\n",
		block.TokensPerMinute(), block.CostPerHour())
	fmt.Println()

	if len(block.Models) > 0 {
		fmt.Printf("  Models: %s\n", strings.Join(block.Models, ", "))
	}
	fmt.Printf("  Sessions: %d • Messages: %d\n", block.SessionCount, block.MessageCount)
	fmt.Println()

	return nil
}

func showRecentBlocks(count int) error {
	blocks, err := db.GetRecentBlocks(count)
	if err != nil {
		return fmt.Errorf("failed to get recent blocks: %w", err)
	}

	if len(blocks) == 0 {
		fmt.Println("No blocks found. Start coding to build history!")
		return nil
	}

	fmt.Printf("━━━ Last %d Block(s) ━━━\n\n", len(blocks))
	printBlocksTable(blocks)

	return nil
}

func showAllBlocks(days int) error {
	blocks, err := db.GetSessionBlocks(days)
	if err != nil {
		return fmt.Errorf("failed to get blocks: %w", err)
	}

	if len(blocks) == 0 {
		fmt.Printf("No blocks found in last %d days.\n", days)
		return nil
	}

	fmt.Printf("━━━ 5-Hour Blocks (Last %d Days) ━━━\n\n", days)
	printBlocksTable(blocks)

	var totalTokens int
	var totalCost float64
	for _, b := range blocks {
		totalTokens += b.TotalTokens()
		totalCost += b.CostUSD
	}

	fmt.Println()
	fmt.Printf("Total: %s tokens • $%.4f across %d blocks\n",
		formatNumber(totalTokens), totalCost, len(blocks))

	return nil
}

func printBlocksTable(blocks []db.SessionBlock) {
	fmt.Println("┌─────────────────────┬──────────────┬──────────┬────────────┬────────┐")
	fmt.Println("│     Time Window     │    Tokens    │   Cost   │  Rate/min  │ Status │")
	fmt.Println("├─────────────────────┼──────────────┼──────────┼────────────┼────────┤")

	for _, b := range blocks {
		status := "      "
		if b.IsActive {
			status = "ACTIVE"
		} else if b.IsGap {
			status = " gap  "
		}

		window := fmt.Sprintf("%s %s-%s",
			b.StartTime.Format("Jan 02"),
			b.StartTime.Format("15:04"),
			b.EndTime.Format("15:04"))

		rate := "-"
		if b.TokensPerMinute() > 0 {
			rate = fmt.Sprintf("%.0f", b.TokensPerMinute())
		}

		fmt.Printf("│ %-19s │ %12s │ $%7.4f │ %10s │ %s │\n",
			window,
			formatNumber(b.TotalTokens()),
			b.CostUSD,
			rate,
			status)
	}

	fmt.Println("└─────────────────────┴──────────────┴──────────┴────────────┴────────┘")
}

func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.2fM", float64(n)/1000000)
}
