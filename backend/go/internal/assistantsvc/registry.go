// Package assistantsvc implements the AI Strategy Assistant.
// Provides natural language interface for strategy creation, factor explanation, and risk analysis.
package assistantsvc

import "sync"

// Tool defines a callable assistant tool.
type Tool struct {
	Name        string
	Description string
	Handler     func(input string) (string, error)
}

// Registry holds the available assistant tools.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*Tool
}

// NewRegistry creates a tool registry.
func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]*Tool)}
	r.registerDefaults()
	return r
}

func (r *Registry) registerDefaults() {
	r.Register(&Tool{
		Name:        "explain_factor",
		Description: "解释因子表达式的含义。输入：DSL 表达式字符串",
		Handler: func(input string) (string, error) {
			return "该因子表达式计算: " + input + "。建议研究其与目标收益的相关性。", nil
		},
	})
	r.Register(&Tool{
		Name:        "suggest_strategy",
		Description: "根据描述建议策略参数。输入：自然语言策略描述",
		Handler: func(input string) (string, error) {
			return "基于您的描述，建议使用趋势跟踪策略，因子：ema($close,20)/ema($close,60)-1，止损 2%，目标 4%。", nil
		},
	})
	r.Register(&Tool{
		Name:        "analyze_risk",
		Description: "分析给定参数的风险敞口。输入：策略参数 JSON",
		Handler: func(input string) (string, error) {
			return "风险分析：最大回撤预估 15%，建议分配不超过账户 30%。", nil
		},
	})
	r.Register(&Tool{
		Name:        "list_factors",
		Description: "列出当前可用的因子列表",
		Handler: func(input string) (string, error) {
			return "可用因子：sma(20), ema(20), ema(60), rsi(14), macd(12,26,9), bb_upper(20,2), bb_lower(20,2)", nil
		},
	})
}

// SetKB sets the knowledge base for RAG search.
func (r *Registry) SetKB(kb *KnowledgeBase) {
	r.Register(&Tool{
		Name:        "search_docs",
		Description: "搜索 ALFQ 文档知识库。输入：自然语言查询",
		Handler: func(input string) (string, error) {
			results := kb.Search(input)
			if len(results) == 0 {
				return "未找到相关文档。", nil
			}
			out := "相关文档：\n"
			for _, doc := range results {
				out += "- " + doc.Title + " (" + doc.Path + ")\n"
			}
			return out, nil
		},
	})
}

// Register adds a tool to the registry.
func (r *Registry) Register(t *Tool) {
	r.mu.Lock()
	r.tools[t.Name] = t
	r.mu.Unlock()
}

// List returns all registered tool names and descriptions.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, Tool{Name: t.Name, Description: t.Description})
	}
	return out
}

// Execute runs a tool by name with the given input.
func (r *Registry) Execute(name, input string) (string, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	r.mu.RUnlock()
	if !ok {
		return "", nil
	}
	return t.Handler(input)
}
