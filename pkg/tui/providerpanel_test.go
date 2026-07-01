package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lcoder/lcoder/pkg/config"
	"github.com/lcoder/lcoder/pkg/events"
	"github.com/lcoder/lcoder/pkg/llm/llmtest"
)

func TestOpenProviderPanelShowsProviderStep(t *testing.T) {
	m, _, _ := newTestModel()
	m.state = stateInput
	m.openProviderPanel()

	if m.state != stateProvider {
		t.Fatalf("expected stateProvider, got %v", m.state)
	}
	if !m.provPanel.visible {
		t.Fatal("expected panel visible")
	}
	if m.provPanel.step != provStepProvider {
		t.Fatalf("expected provStepProvider, got %v", m.provPanel.step)
	}
	if len(m.provPanel.providers) != len(BuiltinProvidersForPanel()) {
		t.Fatalf("expected %d providers, got %d", len(BuiltinProvidersForPanel()), len(m.provPanel.providers))
	}
}

func TestProviderStepNavigationAndEsc(t *testing.T) {
	m, _, _ := newTestModel()
	m.openProviderPanel()

	// Down moves the selection.
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.provPanel.provIdx != 1 {
		t.Fatalf("expected provIdx 1 after down, got %d", m.provPanel.provIdx)
	}

	// Up at top clamps to 0.
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.provPanel.provIdx != 0 {
		t.Fatalf("expected provIdx 0 (clamped), got %d", m.provPanel.provIdx)
	}

	// Esc closes the panel back to input.
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.provPanel.visible || m.state != stateInput {
		t.Fatalf("expected closed panel + stateInput, got visible=%v state=%v", m.provPanel.visible, m.state)
	}
}

func TestModelStepFetchFiltersByProvider(t *testing.T) {
	m, _, _ := newTestModel()
	m.llmClient = llmtest.Client()
	m.openProviderPanel()
	// Provider 0 is openai per BuiltinProviders order.
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if m.provPanel.step != provStepModel {
		t.Fatalf("expected provStepModel, got %v", m.provPanel.step)
	}
	if len(m.provPanel.models) != 1 || m.provPanel.models[0] != "gpt-4o" {
		t.Fatalf("expected [gpt-4o], got %v", m.provPanel.models)
	}
}

func TestModelStepManualFallbackWhenEmpty(t *testing.T) {
	m, _, _ := newTestModel()
	m.llmClient = nil       // no discovery source
	m.cfg = config.Config{} // no catalog fallback either
	m.openProviderPanel()
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !m.provPanel.manual {
		t.Fatal("expected manual model entry when no models discovered")
	}
}

func TestModelStepEnterAdvancesToKey(t *testing.T) {
	m, _, _ := newTestModel()
	m.llmClient = llmtest.Client()
	m.openProviderPanel()
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> model
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // model -> key

	if m.provPanel.step != provStepKey {
		t.Fatalf("expected provStepKey, got %v", m.provPanel.step)
	}
	if m.provPanel.chosenModel != "gpt-4o" {
		t.Fatalf("expected chosenModel gpt-4o, got %q", m.provPanel.chosenModel)
	}
}

func TestCommitProviderSavesRegistersAndSwitches(t *testing.T) {
	m, agent, _ := newTestModel()
	m.llmClient = llmtest.Client()
	// Persist credentials to a temp HOME so we do not touch the real file.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())

	m.openProviderPanel()
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // provider -> model
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // model -> key
	// Type a key and submit.
	for _, r := range "sk-test" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit

	if m.state != stateInput || m.provPanel.visible {
		t.Fatalf("expected panel closed after commit, state=%v visible=%v", m.state, m.provPanel.visible)
	}
	if agent.switchedModel.ID != "gpt-4o" || agent.switchedModel.Provider != "openai" {
		t.Fatalf("expected agent switched to openai/gpt-4o, got %+v", agent.switchedModel)
	}
	if agent.switchedBudget.MaxTotal != 128000 {
		t.Fatalf("expected budget MaxTotal 128000 from catalog window, got %d", agent.switchedBudget.MaxTotal)
	}
	if m.model != "openai/gpt-4o" {
		t.Fatalf("expected display model openai/gpt-4o, got %q", m.model)
	}
}

func TestSlashProviderOpensPanel(t *testing.T) {
	m, _, _ := newTestModel()
	m.state = stateInput
	m.dispatchSlash("/provider")

	if m.state != stateProvider || !m.provPanel.visible {
		t.Fatalf("expected provider panel open, state=%v visible=%v", m.state, m.provPanel.visible)
	}
}

func TestFirstLaunchAutoOpensPanel(t *testing.T) {
	bus := events.New()
	store := &fakeSessionStore{}
	m := NewModel(bus, &fakeAgent{}, &fakeSession{id: "x"}, store, ".", "x",
		"openai/gpt-4o-mini", "dark", nil, nil, nil, nil, config.Config{}, nil, true /* needsProviderSetup */)
	defer m.Close()

	if m.state != stateProvider || !m.provPanel.visible {
		t.Fatalf("expected wizard auto-open on first launch, state=%v visible=%v", m.state, m.provPanel.visible)
	}
}
