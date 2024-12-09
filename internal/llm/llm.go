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

func QueryForReference(ctx context.Context, model llms.Model, linterOutput string) (string, error) {
	log := util.FromContext(ctx)
	if model == nil {
		return "", ErrModelIsNil
	}
	ragQuery := fmt.Sprintf(referenceTemplateStr, linterOutput)

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

const referenceTemplateStr = `
You are a lint expert, you can explain in detail the meaning of the lint result according to the content of the given context.
You will follow the format of the example in <format> to answer in Chinese, firstly, you will explain the lint, secondly, you will give the incorrect usage (it can be a code example or text description), and finally, you will give the correct usage (it can be a code example or a text description), and finally add a blank line before the result.
Please make sure that the output must not exceed 10,000 characters.

<format>

### lint 解释
- 该 lint 出现表示一个变量被赋值后又被重新赋值，但在此过程中没有使用其原始值。这种情况通常表明代码中可能存在冗余，或者可能是逻辑错误，因为原始值在此期间没有被实际使用。
### 错误用法
	// 
	func main() {
		x := 10
		x = 20 // 重新赋值，但未使用原始值

		fmt.Println("x =", x) // 仅打印新值20
	}


### 正确用法
- 方案一：使用赋值变量
- 方案二 ：移除冗余赋值

</format>
Context:
%s

`
