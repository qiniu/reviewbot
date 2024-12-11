package llm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/qiniu/reviewbot/internal/util"
	"github.com/tmc/langchaingo/llms"
)

type Config struct {
	Provider  string
	APIKey    string
	Model     string
	ServerURL string
}

var ErrUnsupportedProvider = errors.New("unsupported llm provider")
var ErrModelIsNil = errors.New("model is nil")

func New(ctx context.Context, config Config) (llms.Model, error) {
	switch config.Provider {
	case "openai":
		return initOpenAIClient(config)
	case "ollama":
		return initOllamaClient(config)
	default:
		return nil, ErrUnsupportedProvider
	}
}

func Query(ctx context.Context, model llms.Model, query string, extraContext string) (string, error) {
	log := util.FromContext(ctx)
	ragQuery := fmt.Sprintf(ragTemplateStr, query, extraContext)

	timeout := 5 * time.Minute
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	respText, err := llms.GenerateFromSinglePrompt(ctxWithTimeout, model, ragQuery)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warnf("LLM query operation timed out after %s", timeout)
		}
		return "", err
	}
	log.Debugf("length of LLM respText: %d", len(respText))
	return respText, nil
}

const ragTemplateStr = `
I will ask you a question and will provide some additional context information.
Assume this context information is factual and correct, as part of internal
documentation.
If the question relates to the context, answer it using the context.
If the question does not relate to the context, answer it as normal.

For example, let's say the context has nothing in it about tropical flowers;
then if I ask you about tropical flowers, just answer what you know about them
without referring to the context.

For example, if the context does mention minerology and I ask you about that,
provide information from the context along with general knowledge.

Question:
%s

Context:
%s
`

func QueryForReference(ctx context.Context, model llms.Model, linterOutput string, codeLanguage string) (string, error) {
	log := util.FromContext(ctx)
	if model == nil {
		return "", ErrModelIsNil
	}
	ragQuery := fmt.Sprintf(referenceTemplateStr, codeLanguage, linterOutput)

	timeout := 5 * time.Minute
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	respText, err := llms.GenerateFromSinglePrompt(ctxWithTimeout, model, ragQuery)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Errorf("LLM query operation timed out after %s", timeout)
		}
		return "", err
	}

	log.Infof("promote linter output:%s, length of response:%d", linterOutput, len(respText))
	return respText, nil
}

const referenceTemplateStr = `
You are a lint expert who can explain in detail the meaning of lint results based on the provided <context>. 

Please follow the format in <format> to respond in Chinese:

1. **lint 解释**:
   - 请仔细查看<context>内容, 其内容为某个具体的lint结果, 请用简短的语言对该lint结果进行解释。
   
2. **错误用法**:
   - 提供一个代码示例或文本描述，展示不正确的用法。若给出代码,代码语言请遵循<codeLanguage>
   
3. **正确用法**:
   - 给出一个代码示例或文本描述，展示正确的用法。若给出代码,代码语言请遵循<codeLanguage>

以上三块内容有且仅输出一次, 即一次“lint 解释”，一次“错误用法”, 一次“正确用法”。保证输出不多不少
请确保输出不超过 5000 个字符


<format>

### lint 解释

### 错误用法
     
### 正确用法

</format>

<codeLanguage>
%s
</codeLanguage>

<context>
%s
</context>
`
