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
	config := agent.Configuration{Version: 2, AgentID: privateAgent.ID, ProjectID: privateAgent.ProjectID, EnvironmentID: privateAgent.EnvironmentID, HeartbeatIntervalSeconds: privateAgent.ExpectedIntervalSeconds, IssuedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
	if s.privateAgentAssignments != nil {
		assignments, listErr := s.privateAgentAssignments.ListPrivateAgentAssignmentsForEnvironment(ctx, privateAgent.ProjectID, privateAgent.EnvironmentID)
		if listErr != nil {
			return agent.SignedConfiguration{}, listErr
		}
		for _, assignment := range assignments {
			if assignment.Enabled && assignment.RevokedAt == nil {
				config.Assignments = append(config.Assignments, agent.Assignment{ID: assignment.ID, Method: assignment.Method, Target: assignment.Target, IntervalSecs: assignment.IntervalSecs, TimeoutMS: assignment.TimeoutMS})
			}
		}
	}
	return s.agentConfigSigner.Sign(config)
}
