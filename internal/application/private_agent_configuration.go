package application

import (
	"context"
	"time"

	"argus/internal/agent"
)

func (s *Service) SetAgentConfigurationSigner(signer *agent.ConfigurationSigner) {
	s.agentConfigSigner = signer
}

func (s *Service) IssuePrivateAgentConfiguration(ctx context.Context, token, version string) (agent.SignedConfiguration, error) {
	if s.agentConfigSigner == nil {
		return agent.SignedConfiguration{}, ErrPrivateAgentNotFound
	}
	privateAgent, err := s.AuthenticatePrivateAgent(ctx, token, version)
	if err != nil {
		return agent.SignedConfiguration{}, err
	}
	now := time.Now().UTC()
	return s.agentConfigSigner.Sign(agent.Configuration{Version: 1, AgentID: privateAgent.ID, ProjectID: privateAgent.ProjectID, EnvironmentID: privateAgent.EnvironmentID, HeartbeatIntervalSeconds: privateAgent.ExpectedIntervalSeconds, IssuedAt: now, ExpiresAt: now.Add(15 * time.Minute)})
}
