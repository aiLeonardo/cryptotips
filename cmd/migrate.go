package cmd

import (
	"fmt"

	"github.com/aiLeonardo/cryptotips/lib"
	"github.com/aiLeonardo/cryptotips/models"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "执行数据库迁移（幂等）",
	RunE: func(cmd *cobra.Command, args []string) error {
		logrusAdapter := lib.NewLogrusAdapter()
		db := lib.LoadDB(logrusAdapter)
		if err := models.EnsureStrategySchema(db); err != nil {
			return err
		}
		fmt.Println("migrate ok: strategy schema ensured")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
