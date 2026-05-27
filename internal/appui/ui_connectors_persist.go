package appui

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/HexmosTech/git-lrc/storage"
)

func persistConnectorsToConfig(configPath string, connectors []aiConnectorRemote) error {
	originalBytes, err := storage.ReadConfigFile(configPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to read config for connector snapshot: %w", err)
		}
		originalBytes = []byte{}
	}

	originalContent := string(originalBytes)
	cleanedContent := stripManagedAIConnectorsSection(originalContent)
	managedSection := renderManagedAIConnectorsSection(connectors)

	trimmed := strings.TrimRight(cleanedContent, "\n\r\t ")
	var updatedContent string
	if trimmed == "" {
		updatedContent = managedSection + "\n"
	} else {
		updatedContent = trimmed + "\n\n" + managedSection + "\n"
	}

	if err := storage.WriteFileAtomically(configPath, []byte(updatedContent), 0600); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func persistAuthTokensToConfig(configPath string, jwt string, refreshToken string) error {
	originalBytes, err := storage.ReadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config for token update: %w", err)
	}

	content := string(originalBytes)
	updated := upsertQuotedConfigValue(content, "jwt", jwt)
	if strings.TrimSpace(refreshToken) != "" {
		updated = upsertQuotedConfigValue(updated, "refresh_token", refreshToken)
	}

	if err := storage.WriteFileAtomically(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func persistOrgContextToConfig(configPath string, orgID string, orgName string, apiKey string) error {
	originalBytes, err := storage.ReadConfigFile(configPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to read config for organization update: %w", err)
		}
		originalBytes = []byte{}
	}

	content := string(originalBytes)
	updated := upsertQuotedConfigValue(content, "org_id", strings.TrimSpace(orgID))
	if strings.TrimSpace(orgName) != "" {
		updated = upsertQuotedConfigValue(updated, "org_name", strings.TrimSpace(orgName))
	}
	if strings.TrimSpace(apiKey) != "" {
		updated = upsertQuotedConfigValue(updated, "api_key", strings.TrimSpace(apiKey))
	}

	if err := storage.WriteFileAtomically(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func persistReauthSessionToConfig(configPath string, apiURL string, result *setupResult) error {
	originalBytes, err := storage.ReadConfigFile(configPath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to read config for reauthentication update: %w", err)
		}
		originalBytes = []byte{}
	}

	updated := string(originalBytes)
	if _, ok := readQuotedConfigValue(updated, "api_url"); !ok {
		if trimmed := strings.TrimSpace(apiURL); trimmed != "" {
			updated = upsertQuotedConfigValue(updated, "api_url", trimmed)
		}
	}

	updates := map[string]string{
		"api_key":         strings.TrimSpace(result.PlainAPIKey),
		"user_email":      strings.TrimSpace(result.Email),
		"user_first_name": strings.TrimSpace(result.FirstName),
		"user_last_name":  strings.TrimSpace(result.LastName),
		"avatar_url":      strings.TrimSpace(result.AvatarURL),
		"user_id":         strings.TrimSpace(result.UserID),
		"org_id":          strings.TrimSpace(result.OrgID),
		"org_name":        strings.TrimSpace(result.OrgName),
		"jwt":             strings.TrimSpace(result.AccessToken),
		"refresh_token":   strings.TrimSpace(result.RefreshToken),
	}

	for key, value := range updates {
		if value == "" {
			continue
		}
		updated = upsertQuotedConfigValue(updated, key, value)
	}

	if err := storage.WriteFileAtomically(configPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}
