package insight

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alfa-hack/alfa-hack/services/insight/internal/agentclient"
)

type Analyzer interface {
	Analyze(ctx context.Context, req agentclient.AnalysisRequest, emit func(agentclient.Event) error) error
}

type AgentAnalyzer struct {
	client  *agentclient.Client
	timeout time.Duration
}

func NewAgentAnalyzer(baseURL string, timeout time.Duration) *AgentAnalyzer {
	return &AgentAnalyzer{client: agentclient.New(baseURL), timeout: timeout}
}

func (a *AgentAnalyzer) Analyze(ctx context.Context, req agentclient.AnalysisRequest, emit func(agentclient.Event) error) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.client.Analyze(ctx, req, emit)
}

type StubAnalyzer struct {
	delay time.Duration
}

func NewStubAnalyzer(delay time.Duration) *StubAnalyzer {
	return &StubAnalyzer{delay: delay}
}

func (a *StubAnalyzer) Analyze(ctx context.Context, req agentclient.AnalysisRequest, emit func(agentclient.Event) error) error {
	steps := []struct {
		name    string
		message string
	}{
		{name: "retrieving", message: "Ищем связанные новости и документы"},
		{name: "ranking", message: "Оцениваем влияние на компанию"},
		{name: "analyzing", message: "Готовим вывод AI и позицию банка"},
	}

	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		data, _ := json.Marshal(map[string]any{
			"type":    "step",
			"name":    step.name,
			"status":  "running",
			"message": step.message,
		})
		if err := emit(agentclient.Event{Type: "step", Data: data}); err != nil {
			return err
		}
		if err := sleep(ctx, a.delay); err != nil {
			return err
		}
	}

	sources := make([]Source, 0, len(req.Evidence))
	for _, evidence := range req.Evidence {
		sources = append(sources, Source{SourceID: evidence.SourceID, Title: evidence.Title, URL: evidence.URL})
	}
	if len(sources) == 0 {
		sources = append(sources, Source{SourceID: req.Event.ID, Title: req.Event.Title, URL: "https://example.com/news/" + req.Event.ID})
	}

	item := Insight{
		Summary:       fmt.Sprintf("%s: %s", req.Company.Name, req.Event.Title),
		WhyItMatters:  "Событие влияет на операционную эффективность и переговорную позицию компании.",
		Risks:         []string{"Рост затрат может ограничить свободный денежный поток.", "Сроки реализации проекта могут увеличиться."},
		Opportunities: []string{"Усиление позиций на ключевом рынке.", "Повод для диалога о финансировании и хеджировании."},
		BankPosition:  "Позиция банка: событие нейтрально-позитивно, но требует проверки долговой нагрузки и параметров сделки.",
		AIConclusion:  "Вывод AI: сигнал умеренно позитивный, эффект зависит от скорости исполнения и стоимости капитала.",
		Sources:       sources,
		Actions:       []string{"Проверить финансовые показатели компании.", "Подготовить сценарий влияния на денежный поток."},
		Confidence:    0.72,
		Meta:          map[string]any{"model": "stub", "prompt_version": "v1"},
	}
	data, _ := json.Marshal(map[string]any{"type": "result", "insight": item})
	return emit(agentclient.Event{Type: "result", Data: data})
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
