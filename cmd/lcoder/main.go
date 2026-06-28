package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/lcoder/lcoder/pkg/agent"
	"github.com/lcoder/lcoder/pkg/agent/hooks"
	"github.com/lcoder/lcoder/pkg/compaction"
	"github.com/lcoder/lcoder/pkg/config"
	contextloader "github.com/lcoder/lcoder/pkg/context"
	"github.com/lcoder/lcoder/pkg/contextmgr"
	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/extension"
	"github.com/lcoder/lcoder/pkg/llm"
	"github.com/lcoder/lcoder/pkg/llm/catalog"
	"github.com/lcoder/lcoder/pkg/llm/engine"
	llmprovider "github.com/lcoder/lcoder/pkg/llm/provider"
	"github.com/lcoder/lcoder/pkg/mcp"
	"github.com/lcoder/lcoder/pkg/models"
	"github.com/lcoder/lcoder/pkg/observability"
	"github.com/lcoder/lcoder/pkg/permissions"
	"github.com/lcoder/lcoder/pkg/session"
	"github.com/lcoder/lcoder/pkg/skills"
	"github.com/lcoder/lcoder/pkg/tools"
	_ "github.com/lcoder/lcoder/pkg/tools/builtin"
	"github.com/lcoder/lcoder/pkg/tui"
)

const (
	defaultMaxTurns    = 25
	compactionKeep     = 10
	compactionInterval = 5
)

var (
	cfgFile    string
	modelID    string
	provider   string
	sessionID  string
	cont       bool
	modeName   string
	promptText string
)

func main() {
	root := &cobra.Command{
		Use:   "lcoder [prompt]",
		Short: "Lcoder — a minimal, extensible SWE agent",
		RunE:  runRoot,
	}

	root.Flags().StringVar(&cfgFile, "config", "", "Path to config file")
	root.Flags().StringVar(&modelID, "model", "", "Model ID")
	root.Flags().StringVar(&provider, "provider", "", "Model provider")
	root.Flags().StringVar(&sessionID, "session", "", "Session ID to resume")
	root.Flags().BoolVarP(&cont, "continue", "c", false, "Continue most recent session")
	root.Flags().StringVar(&modeName, "mode", "", "Agent mode: plan, code, explore, review, test")
	root.Flags().StringVarP(&promptText, "prompt", "p", "", "Single prompt to run and exit")
	root.Flags().Bool("json", false, "Output events as JSONL instead of TUI/text")

	root.AddCommand(modelsCmd())
	root.AddCommand(skillsCmd())
	root.AddCommand(sessionsCmd())
	root.AddCommand(forkCmd())
	root.AddCommand(cloneCmd())
	root.AddCommand(modesCmd())
	root.AddCommand(statsCmd())
	root.AddCommand(exportCmd())
	root.AddCommand(traceCmd())
	root.AddCommand(metricsCmd())
	root.AddCommand(tuiCmd())
	root.AddCommand(installCmd())
	root.AddCommand(uninstallCmd())
	root.AddCommand(listExtensionsCmd())
	root.AddCommand(updateCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadConfig() (config.Config, error) {
	cfg := config.DefaultConfig()
	if cfgFile != "" {
		data, err := os.ReadFile(cfgFile)
		if err != nil {
			return cfg, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	} else {
		loaded, err := config.Load()
		if err != nil {
			return cfg, err
		}
		cfg = loaded
	}
	if modelID != "" {
		cfg.Model = modelID
	}
	if provider != "" {
		cfg.Provider = provider
	}
	return cfg, nil
}

type agentSetup struct {
	ag          *agent.Agent
	sess        *session.Session
	store       *session.Store
	bus         *events.Bus
	mcpRegistry *mcp.Registry
	cfg         agentConfig
	cwd         string
	llmClient   *llm.Client
	cleanup     func()
}

type agentConfig struct {
	config.Config
	loadedSkills []skills.Skill
	modeManager  *agent.ModeManager
}

func prepareAgent(cfg config.Config, cwd string) (*agentSetup, error) {
	ctxLoader := contextloader.NewLoader(cwd)
	contextText, err := ctxLoader.Load()
	if err != nil {
		return nil, err
	}

	extMgr := extension.DefaultManager()

	skillPaths := append(skills.DefaultPaths(cwd), extMgr.SkillDirs()...)
	loadedSkills, _ := skills.Load(skillPaths)
	skillsBlock := skills.ToSystemPromptBlock(loadedSkills)

	// Non-fatal capability check: warn if the configured model is known not to
	// support tool calling, since the agent relies on tools.
	if cfg.ModelLacksTools() {
		fmt.Fprintf(os.Stderr, "warning: model %q does not declare the \"tools\" capability; tool calls may fail\n", cfg.Model)
	}

	llmClient := llm.NewClient(buildEngine(cfg))

	registry := tools.NewRegistry(cwd)
	if err := registry.RegisterBuiltinFactories(cwd); err != nil {
		return nil, fmt.Errorf("register built-in tools: %w", err)
	}
	for _, cfgTool := range cfg.HTTPTools {
		registry.Register(cfgTool.Name, tools.NewHTTPExecutable(tools.HTTPConfig{
			Name:          cfgTool.Name,
			Endpoint:      tools.ExpandEndpointEnv(cfgTool.Endpoint),
			Description:   cfgTool.Description,
			Parameters:    cfgTool.Parameters,
			ExecutionMode: models.ExecutionMode(cfgTool.ExecutionMode),
			Headers:       cfgTool.Headers,
		}))
	}

	mcpConfigs := make([]mcp.ServerConfig, 0, len(cfg.MCPServers))
	for _, s := range cfg.MCPServers {
		mcpConfigs = append(mcpConfigs, mcp.ServerConfig{
			Name:    s.Name,
			Command: s.Command,
			Env:     s.Env,
		})
	}
	mcpRegistry := mcp.NewRegistry(mcpConfigs)
	if err := mcpRegistry.Connect(); err != nil {
		return nil, fmt.Errorf("mcp connect: %w", err)
	}
	mcpRegistry.RegisterTools(registry)

	permEngine := permissions.NewEngineFromRules(parsePermissionConfig(cfg.Permissions))

	sessStore := session.NewStore("")
	var sess *session.Session
	if sessionID != "" {
		sess, err = sessStore.LoadByID(cwd, sessionID)
		if err != nil {
			mcpRegistry.Close()
			return nil, fmt.Errorf("load session: %w", err)
		}
	} else if cont {
		sess, err = sessStore.MostRecent(cwd)
		if err != nil {
			mcpRegistry.Close()
			return nil, fmt.Errorf("continue session: %w", err)
		}
	} else {
		sess, err = sessStore.Create(cwd)
		if err != nil {
			mcpRegistry.Close()
			return nil, fmt.Errorf("create session: %w", err)
		}
	}

	bus := events.New()
	obsExporter, err := observability.NewFileExporter(observability.DefaultPath(sess.ID))
	if err != nil {
		mcpRegistry.Close()
		return nil, fmt.Errorf("observability exporter: %w", err)
	}
	auditLogger, err := observability.NewFileAuditLogger(observability.DefaultAuditPath(sess.ID))
	if err != nil {
		mcpRegistry.Close()
		return nil, fmt.Errorf("audit logger: %w", err)
	}
	obsCollector := observability.NewCollectorWithAudit(obsExporter, sess.ID, auditLogger)
	obsCollector.Subscribe(bus)

	modeManager := agent.NewModeManager()
	modeDirs := append(agent.DefaultModeDirs(cwd), extMgr.AgentDirs()...)
	_ = modeManager.LoadModes(modeDirs)
	if modeName == "" {
		modeName = "code"
	}

	window, _ := llmClient.ModelWindow(context.Background(), cfg.Provider, cfg.Model)
	budget, source := cfg.ResolveContextBudget(window)
	if source == "default" {
		fmt.Fprintf(os.Stderr, "warning: 未能自动获取模型 %q 的上下文窗口,回退默认 %d\n", cfg.Model, budget.MaxTotal)
	}
	mgr := makeContextManager(cfg, budget, llmClient, contextText, skillsBlock, sess.ActiveMessages())
	ag, err := agent.NewBuilder().
		WithConfig(agent.Config{
			SystemPrompt:      "",
			Model:             models.ModelRef{Provider: cfg.Provider, ID: cfg.Model},
			MaxTurns:          defaultMaxTurns,
			ToolExecutionMode: models.ExecutionParallel,
			ContextManager:    mgr,
			BeforeToolCall:    makeBeforeToolCall(bus, permEngine, cfg.Hooks),
			Mode:              modeName,
			ModeManager:       modeManager,
		}).
		WithGatewayClient(llmClient).
		WithRegistry(registry).
		WithPermissions(permEngine).
		WithEventBus(bus).
		WithObservability(obsCollector).
		Build()
	if err != nil {
		mcpRegistry.Close()
		return nil, fmt.Errorf("build agent: %w", err)
	}
	ag.SetMessages(sess.ActiveMessages())

	return &agentSetup{
		ag:          ag,
		sess:        sess,
		store:       sessStore,
		bus:         bus,
		mcpRegistry: mcpRegistry,
		cfg:         agentConfig{Config: cfg, loadedSkills: loadedSkills},
		cwd:         cwd,
		llmClient:   llmClient,
		cleanup: func() {
			obsCollector.Close()
			mcpRegistry.Close()
		},
	}, nil
}

func runRoot(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	setup, err := prepareAgent(cfg, cwd)
	if err != nil {
		return err
	}
	defer setup.cleanup()

	if promptText == "" && len(args) > 0 {
		promptText = strings.Join(args, " ")
	}

	jsonMode, _ := cmd.Flags().GetBool("json")
	if jsonMode {
		return runJSONMode(cmd.Context(), setup, promptText)
	}

	if promptText != "" {
		return runOneShot(cmd.Context(), setup, promptText)
	}

	return runTUI(cmd.Context(), setup)
}

func runJSONMode(ctx context.Context, setup *agentSetup, prompt string) error {
	var msg models.AgentMessage
	if prompt != "" {
		msg = models.NewAgentMessage(models.RoleUser, models.TextContent{Text: prompt})
		if err := setup.sess.Append(msg); err != nil {
			return fmt.Errorf("append message: %w", err)
		}
	}

	jsonHandler := func(ctx context.Context, ev events.Event) error {
		data, err := events.MarshalJSON(ev)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	unsub := setup.bus.Subscribe(jsonHandler)
	defer unsub()

	if prompt != "" {
		if err := setup.ag.Prompt(ctx, msg); err != nil {
			return err
		}
	} else {
		if err := setup.ag.Continue(ctx); err != nil {
			return err
		}
	}
	return nil
}

func runOneShot(ctx context.Context, setup *agentSetup, prompt string) error {
	fmt.Printf("[lcoder] session=%s mode=%s\n", setup.sess.ID, modeName)

	// Persist after each assistant/tool message turn.
	persistHandler := func(ctx context.Context, ev events.Event) error {
		switch ev.(type) {
		case events.MessageEndEvent, events.ToolExecutionEndEvent, events.AgentEndEvent:
			_ = setup.sess.Save()
		}
		return nil
	}
	unsub := setup.bus.Subscribe(persistHandler)
	defer unsub()

	var initialMessages []models.AgentMessage
	if name, rest, ok := skills.ParseManualTrigger(prompt); ok {
		if skill, found := skills.FindByName(setup.cfg.loadedSkills, name); found {
			initialMessages = skills.ExpandManualTrigger(skill, rest)
		} else {
			return fmt.Errorf("skill %q not found", name)
		}
	} else if setup.cfg.Context.Mode == "auto" {
		// Auto-detect skill from prompt when no manual trigger is used.
		if score, ok := skills.AutoDetect(prompt, setup.cfg.loadedSkills); ok {
			fmt.Printf("[lcoder] auto-activated skill: %s\n", score.Skill.Name)
			initialMessages = skills.ExpandManualTrigger(score.Skill, prompt)
		}
	}

	var msg models.AgentMessage
	if len(initialMessages) > 0 {
		for _, m := range initialMessages {
			if err := setup.sess.Append(m); err != nil {
				return fmt.Errorf("append message: %w", err)
			}
		}
		msg = initialMessages[len(initialMessages)-1]
	} else {
		msg = models.NewAgentMessage(models.RoleUser, models.TextContent{Text: prompt})
		if err := setup.sess.Append(msg); err != nil {
			return fmt.Errorf("append message: %w", err)
		}
	}

	if err := setup.ag.Prompt(ctx, msg); err != nil {
		return err
	}
	final := setup.ag.AllMessages()
	if len(final) == 0 {
		return nil
	}
	fmt.Println(final[len(final)-1].Text())
	return nil
}

func runTUI(ctx context.Context, setup *agentSetup) error {
	httpTools := make([]tui.HTTPToolItem, 0, len(setup.cfg.HTTPTools))
	for _, t := range setup.cfg.HTTPTools {
		httpTools = append(httpTools, tui.HTTPToolItem{
			Name:        t.Name,
			Endpoint:    t.Endpoint,
			Description: t.Description,
		})
	}
	modelRef := setup.cfg.Provider + "/" + setup.cfg.Model

	// Persist after each assistant/tool message turn.
	persistHandler := func(ctx context.Context, ev events.Event) error {
		switch ev.(type) {
		case events.MessageEndEvent, events.ToolExecutionEndEvent, events.AgentEndEvent:
			_ = setup.sess.Save()
		}
		return nil
	}
	unsub := setup.bus.Subscribe(persistHandler)
	defer unsub()

	var caps []string
	if meta, ok := setup.cfg.ModelMeta(); ok {
		caps = meta.Capabilities
	}
	needsSetup := !config.ProviderHasKey(setup.cfg.Config, setup.cfg.Provider)
	return tui.Run(setup.bus, setup.ag, setup.sess, setup.store, setup.cwd, modelRef, setup.cfg.TUI.Theme, httpTools, setup.mcpRegistry, setup.cfg.modeManager, caps, setup.llmClient, setup.cfg.Config, needsSetup, setup.cfg.loadedSkills...)
}

func modelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "List available models from the catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			client := llm.NewClient(buildEngine(cfg))
			models, err := client.ListModels(cmd.Context())
			if err != nil {
				return err
			}
			for _, m := range models {
				fmt.Println(m.ID)
			}
			return nil
		},
	}
}

func skillsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "skills",
		Short: "List loaded skills and their prompts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			paths := append(skills.DefaultPaths(cwd), extension.DefaultManager().SkillDirs()...)
			loaded, err := skills.Load(paths)
			if err != nil {
				return err
			}
			for _, s := range loaded {
				fmt.Printf("- %s (%s)\n", s.Name, s.Source)
			}
			return nil
		},
	}
}

func sessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List sessions for the current workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			store := session.NewStore("")
			sessions, err := store.List(cwd)
			if err != nil {
				return err
			}
			for _, s := range sessions {
				fmt.Printf("%s %s\n", s.ID, time.Unix(s.CreatedAt, 0).Format("2006-01-02 15:04"))
			}
			return nil
		},
	}
}

func forkCmd() *cobra.Command {
	var parentID, messageID string
	cmd := &cobra.Command{
		Use:   "fork --session SESSION --message MESSAGE_ID",
		Short: "Fork a session at a specific message",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			store := session.NewStore("")
			parent, err := store.LoadByID(cwd, parentID)
			if err != nil {
				return err
			}
			forked, err := store.Fork(cwd, parent, messageID)
			if err != nil {
				return err
			}
			fmt.Println(forked.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&parentID, "session", "", "Parent session ID")
	cmd.Flags().StringVar(&messageID, "message", "", "Message ID to fork at")
	_ = cmd.MarkFlagRequired("session")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

func cloneCmd() *cobra.Command {
	var sourceID string
	cmd := &cobra.Command{
		Use:   "clone --session SESSION",
		Short: "Create a full clone of a session",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			store := session.NewStore("")
			source, err := store.LoadByID(cwd, sourceID)
			if err != nil {
				return err
			}
			cloned, err := store.Clone(cwd, source)
			if err != nil {
				return err
			}
			fmt.Println(cloned.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&sourceID, "session", "", "Session ID to clone")
	_ = cmd.MarkFlagRequired("session")
	return cmd
}

func modesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "modes",
		Short: "List available agent modes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			mm := agent.NewModeManager()
			dirs := append(agent.DefaultModeDirs(cwd), extension.DefaultManager().AgentDirs()...)
			_ = mm.LoadModes(dirs)
			for _, mode := range mm.List() {
				fmt.Printf("- %s: %s\n", mode.Name, mode.Description)
			}
			return nil
		},
	}
}

func statsCmd() *cobra.Command {
	var obsPath string
	cmd := &cobra.Command{
		Use:   "stats --session SESSION",
		Short: "Show observability stats for a session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if obsPath == "" {
				if len(args) == 0 {
					return fmt.Errorf("session id required")
				}
				obsPath = observability.DefaultPath(args[0])
			}
			stats, err := observability.SummarizeFile(obsPath)
			if err != nil {
				return err
			}
			fmt.Printf("turns: %d\n", stats.Turns)
			fmt.Printf("input tokens: %d\n", stats.InputTokens)
			fmt.Printf("output tokens: %d\n", stats.OutputTokens)
			fmt.Printf("total cost: $%.6f\n", stats.TotalCost)
			return nil
		},
	}
	cmd.Flags().StringVar(&obsPath, "file", "", "Observability JSONL file")
	return cmd
}

func exportCmd() *cobra.Command {
	var format, output string
	cmd := &cobra.Command{
		Use:   "export --session SESSION",
		Short: "Export session observability to html/sqlite/prometheus",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("session id required")
			}
			path := observability.DefaultPath(args[0])
			var exporter observability.Exporter
			switch format {
			case "sqlite":
				var err error
				exporter, err = observability.NewSQLiteExporter(output)
				if err != nil {
					return err
				}
			case "html":
				exporter = observability.NewHTMLExporter()
			case "prometheus":
				exporter = observability.NewPrometheusExporter()
			default:
				return fmt.Errorf("unknown format: %s", format)
			}
			return observability.ExportFile(path, exporter, output)
		},
	}
	cmd.Flags().StringVar(&format, "format", "html", "Export format: html, sqlite, prometheus")
	cmd.Flags().StringVarP(&output, "output", "o", "lcoder-export", "Output file or directory")
	return cmd
}

func traceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trace SESSION",
		Short: "Print formatted trace for a session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("session id required")
			}
			path := observability.DefaultPath(args[0])
			trace, err := observability.FormatTrace(path)
			if err != nil {
				return err
			}
			fmt.Println(trace)
			return nil
		},
	}
}

func metricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Run Prometheus metrics endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			port := "9090"
			if len(args) > 0 {
				port = args[0]
			}
			return observability.ServeMetrics(port)
		},
	}
	return cmd
}

func tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Start interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg, err := loadConfig()
			if err != nil {
				return fmt.Errorf("config: %w", err)
			}
			setup, err := prepareAgent(cfg, cwd)
			if err != nil {
				return err
			}
			defer setup.cleanup()
			httpTools := make([]tui.HTTPToolItem, 0, len(cfg.HTTPTools))
			for _, t := range cfg.HTTPTools {
				httpTools = append(httpTools, tui.HTTPToolItem{
					Name:        t.Name,
					Endpoint:    t.Endpoint,
					Description: t.Description,
				})
			}
			modelRef := cfg.Provider + "/" + cfg.Model
			var caps []string
			if meta, ok := cfg.ModelMeta(); ok {
				caps = meta.Capabilities
			}
			needsSetup := !config.ProviderHasKey(cfg, cfg.Provider)
			return tui.Run(setup.bus, setup.ag, setup.sess, setup.store, cwd, modelRef, cfg.TUI.Theme, httpTools, setup.mcpRegistry, setup.cfg.modeManager, caps, setup.llmClient, cfg, needsSetup, setup.cfg.loadedSkills...)
		},
	}
}

func installCmd() *cobra.Command {
	var name string
	var local bool
	cmd := &cobra.Command{
		Use:   "install SOURCE",
		Short: "Install an extension or package from a local path or git repo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if local {
				abs, err := filepath.Abs(source)
				if err != nil {
					return err
				}
				source = abs
			}
			if name == "" {
				name = guessName(source)
			}
			mgr := extension.DefaultManager()
			pkg, err := mgr.InstallPackage(name, source)
			if err != nil {
				// Try as a Go extension.
				loader := extension.DefaultLoader()
				dir, err2 := loader.Install(name, source)
				if err2 != nil {
					return fmt.Errorf("install package: %w\ninstall extension: %w", err, err2)
				}
				fmt.Printf("Installed extension %s at %s\n", name, dir)
				return nil
			}
			fmt.Printf("Installed package %s v%s at %s\n", pkg.Info.Name, pkg.Info.Version, pkg.RootDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Name for the installed extension/package")
	cmd.Flags().BoolVarP(&local, "local", "l", false, "Force treat SOURCE as a local path")
	return cmd
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall NAME",
		Short: "Uninstall an extension or package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			mgr := extension.DefaultManager()
			var errs []error
			if err := mgr.UninstallPackage(name); err != nil {
				errs = append(errs, err)
			}
			if err := extension.DefaultLoader().Uninstall(name); err != nil {
				errs = append(errs, err)
			}
			if len(errs) == 2 {
				return fmt.Errorf("uninstall failed: %v", errs)
			}
			fmt.Printf("Uninstalled %s\n", name)
			return nil
		},
	}
}

func listExtensionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list-extensions",
		Short: "List installed extensions and packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := extension.DefaultManager()
			pkgs, err := mgr.ListPackages()
			if err != nil {
				return err
			}
			if len(pkgs) > 0 {
				fmt.Println("Packages:")
				for _, p := range pkgs {
					fmt.Printf("- %s v%s (%s)\n", p.Info.Name, p.Info.Version, p.RootDir)
				}
			}
			exts, err := extension.DefaultLoader().List()
			if err != nil {
				return err
			}
			if len(exts) > 0 {
				fmt.Println("Extensions:")
				for _, e := range exts {
					fmt.Printf("- %s\n", e)
				}
			}
			if len(pkgs) == 0 && len(exts) == 0 {
				fmt.Println("No extensions or packages installed.")
			}
			return nil
		},
	}
}

func updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update NAME",
		Short: "Update an installed extension or package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			// For local extensions, update is a no-op. For git, reinstall.
			loader := extension.DefaultLoader()
			dir, err := loader.LoadPath(name)
			if err != nil {
				return fmt.Errorf("extension not found: %w", err)
			}
			// Detect if it's a git repo.
			gitDir := filepath.Join(dir, ".git")
			if _, err := os.Stat(gitDir); err == nil {
				if err := loader.Uninstall(name); err != nil {
					return err
				}
				// Re-clone from existing remote.
				out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
				if err != nil {
					return fmt.Errorf("get remote url: %w", err)
				}
				remote := strings.TrimSpace(string(out))
				newDir, err := loader.Install(name, remote)
				if err != nil {
					return err
				}
				fmt.Printf("Updated extension %s at %s\n", name, newDir)
				return nil
			}
			fmt.Printf("%s is a local extension; update is not supported. Reinstall if needed.\n", name)
			return nil
		},
	}
}

func guessName(source string) string {
	base := filepath.Base(source)
	base = strings.TrimSuffix(base, ".git")
	if base == "" || base == "." {
		return "unknown"
	}
	return base
}

// buildEngine constructs the in-process LLM engine: a model catalog (snapshot +
// background refresh + models.yaml overrides) plus every configured provider
// connection. The returned engine is passed to llm.NewClient.
func buildEngine(cfg config.Config) *engine.Engine {
	cachePath := ""
	if home, err := os.UserHomeDir(); err == nil {
		cachePath = filepath.Join(home, ".lcoder", "cache", "models.json")
	}
	cat := catalog.New(catalog.Options{
		Refresh:   true,
		CachePath: cachePath,
		Overrides: catalogOverridesFromConfig(cfg),
	})
	eng := engine.New(cat)
	for name, conn := range cfg.Providers {
		eng.RegisterProvider(name, llmprovider.Conn{
			BaseURL: conn.BaseURL,
			APIKey:  conn.APIKey,
			Route:   conn.Route,
			Headers: conn.Headers,
		})
	}
	return eng
}

// catalogOverridesFromConfig maps the user's models.yaml catalog entries into
// explicit catalog overrides so locally-declared models take priority over the
// snapshot/models.dev data.
func catalogOverridesFromConfig(cfg config.Config) []catalog.Entry {
	out := make([]catalog.Entry, 0, len(cfg.Catalog.Models))
	for _, m := range cfg.Catalog.Models {
		out = append(out, catalog.Entry{
			ID:            m.ID,
			Provider:      m.Provider,
			ContextWindow: m.ContextWindow,
			Capabilities:  m.Capabilities,
		})
	}
	return out
}

func buildSystemPrompt(contextText, skillsBlock string) string {
	var b strings.Builder
	b.WriteString("You are Lcoder, an expert software engineering agent.\n")
	if contextText != "" {
		b.WriteString("\n")
		b.WriteString(contextText)
	}
	if skillsBlock != "" {
		b.WriteString("\n")
		b.WriteString(skillsBlock)
	}
	b.WriteString("\n\nUse tools naturally. Prefer parallel tool calls when independent. Report concise progress and final results.")
	return b.String()
}

func makeContextManager(cfg config.Config, budget config.TokenBudget, llmClient *llm.Client, contextText, skillsBlock string, activeMessages []models.AgentMessage) *contextmgr.Manager {
	opts := []contextmgr.Option{
		contextmgr.WithWindowPolicy(contextmgr.NewKeepRecentInBudget(cfg.Context.MinRecent)),
	}
	// Attach a real LLM summarizer (guarded by a circuit breaker) only when
	// automatic compaction is enabled. Otherwise the window policy degrades to
	// truncation. The breaker trips after repeated failures so a flaky provider
	// never crashes the turn.
	if cfg.Context.AutoCompact && cfg.Context.Mode == "auto" {
		breaker := compaction.NewCircuitBreaker(0)
		summarizer := compaction.NewLLMSummarizer(llmClient, models.ModelRef{Provider: cfg.Provider, ID: cfg.Model})
		opts = append(opts, contextmgr.WithSummarizer(contextmgr.SummarizeFunc(breaker.Wrap(summarizer))))
	}

	mgr := contextmgr.NewManager(contextmgr.TokenBudget{
		MaxTotal:         budget.MaxTotal,
		TargetTotal:      budget.TargetTotal,
		ReserveOutput:    budget.ReserveOutput,
		CompactThreshold: budget.CompactThreshold,
	}, opts...)

	systemText := buildSystemPrompt(contextText, skillsBlock)
	mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockSystem, "system", contextmgr.StabilityStatic, 100,
		models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: systemText})))

	if contextText != "" {
		mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockProjectDocs, "project_docs", contextmgr.StabilityStable, 80,
			models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: contextText})))
	}

	if skillsBlock != "" {
		mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockSkills, "skills", contextmgr.StabilityStable, 90,
			models.NewAgentMessage(models.RoleSystem, models.TextContent{Text: skillsBlock})))
	}

	if len(activeMessages) > 0 {
		mgr.SetBlock(contextmgr.NewBlock(contextmgr.BlockRecent, "recent", contextmgr.StabilityDynamic, 100, activeMessages...))
	}

	return mgr
}

func makeTransformContext(keep int) agent.TransformContext {
	strategy := compaction.NewKeepLastStrategy(keep)
	return func(ctx context.Context, messages []models.AgentMessage) ([]models.AgentMessage, error) {
		if len(messages) <= keep+1 {
			return messages, nil
		}
		if len(messages)%compactionInterval == 0 {
			return strategy.Compact(messages, compaction.SimpleSummarize)
		}
		return messages, nil
	}
}

func makeBeforeToolCall(bus *events.Bus, engine *permissions.Engine, hookCfg config.HookConfig) agent.BeforeToolCallHook {
	declarative := hooks.FromConfig(hookCfg)
	permissionHook := func(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
		decision := engine.Decide(info.ToolCall.Name, info.Args)

		// Emit audit event for every permission decision.
		var blocked bool
		var blockReason string
		allowed := decision == permissions.Allow
		if decision == permissions.Deny {
			blocked = true
			blockReason = "denied by policy"
		}

		_ = bus.Emit(ctx, events.AuditEvent{
			Base:        events.Base{Type: events.Audit},
			ToolCallID:  info.ToolCall.ID,
			ToolName:    info.ToolCall.Name,
			Args:        info.Args,
			Decision:    string(decision),
			Allowed:     allowed,
			Blocked:     blocked,
			BlockReason: blockReason,
		})

		switch decision {
		case permissions.Allow:
			return nil, nil
		case permissions.Deny:
			return &agent.BeforeToolCallResult{Block: true, Reason: blockReason}, nil
		case permissions.Ask:
			result, err := askUser(ctx, info)
			if err != nil {
				return nil, err
			}
			// Emit follow-up audit event reflecting the interactive decision.
			askBlockReason := ""
			if result != nil {
				askBlockReason = result.Reason
			}
			_ = bus.Emit(ctx, events.AuditEvent{
				Base:        events.Base{Type: events.Audit},
				ToolCallID:  info.ToolCall.ID,
				ToolName:    info.ToolCall.Name,
				Args:        info.Args,
				Decision:    "ask",
				Allowed:     result == nil || !result.Block,
				Blocked:     result != nil && result.Block,
				BlockReason: askBlockReason,
			})
			return result, nil
		default:
			return nil, nil
		}
	}

	return hooks.CompositeBeforeToolCall(declarative, permissionHook)
}

func askUser(ctx context.Context, info agent.ToolCallInfo) (*agent.BeforeToolCallResult, error) {
	fmt.Fprintf(os.Stderr, "\nPermission request: %s(%s)\nAllow? [y/N] ", info.ToolCall.Name, formatArgs(info.Args))
	var line string
	fmt.Fscanln(os.Stdin, &line)
	if strings.EqualFold(strings.TrimSpace(line), "y") {
		return nil, nil
	}
	return &agent.BeforeToolCallResult{Block: true, Reason: "user denied"}, nil
}

func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}

func parsePermissionConfig(pc config.PermissionConfig) []permissions.Rule {
	var rules []permissions.Rule
	for tool, patterns := range pc.Rules {
		for pattern, decision := range patterns {
			rules = append(rules, permissions.Rule{
				Tool:     tool,
				Pattern:  pattern,
				Decision: permissions.Decision(decision),
			})
		}
	}
	return rules
}
