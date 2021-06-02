package questionnaire

import (
	"fmt"
	"github.com/avast/retry-go"
	"github.com/codefresh-io/argocd-listener/installer/pkg/logger"
	argoSdk "github.com/codefresh-io/argocd-sdk/pkg/api"
	"time"
)

type ArgocdTokenQuestion struct {
	username string
	password string
	argoHost string
}

func NewArgocdTokenQuestion(username, password, argoHost string) *ArgocdTokenQuestion {
	return &ArgocdTokenQuestion{
		username: username,
		password: password,
		argoHost: argoHost,
	}
}

func (tokenQuestion *ArgocdTokenQuestion) Ask() (string, error) {
	var token string
	var err error

	err = retry.Do(
		func() error {
			token, err = argoSdk.GetToken(tokenQuestion.username, tokenQuestion.password, tokenQuestion.argoHost)
			if err != nil {
				return err
			}
			return nil
		},
		retry.DelayType(func(n uint, err error, config *retry.Config) time.Duration {
			logger.Warning(fmt.Sprintf("Getting argocd token failed, reason \"%v\", attempt %v ", err, n+1))
			return retry.BackOffDelay(n, err, config)
		}),
		retry.Delay(5*time.Second),
	)
	return token, err
}
