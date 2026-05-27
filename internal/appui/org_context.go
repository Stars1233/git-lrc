package appui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/HexmosTech/git-lrc/network"
	uicfg "github.com/HexmosTech/git-lrc/ui"
)

type organizationsListResponse struct {
	Organizations []organizationRow `json:"organizations"`
}

type organizationRow struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	RoleName string `json:"role_name"`
}

type sessionOrgsResult struct {
	Organizations []uicfg.SessionOrganization
	JWT           string
	StatusCode    int
}

func (s *connectorManagerServer) fetchSessionOrganizations() (*sessionOrgsResult, error) {
	s.mu.Lock()
	rawJWT := s.cfg.JWT
	rawAPIURL := s.cfg.APIURL
	s.mu.Unlock()

	jwt := strings.TrimSpace(rawJWT)
	apiURL := strings.TrimSpace(rawAPIURL)

	if jwt == "" {
		return &sessionOrgsResult{JWT: "", StatusCode: http.StatusUnauthorized, Organizations: []uicfg.SessionOrganization{}}, nil
	}

	status, body, err := s.getOrganizationsWithBearer(jwt, apiURL)
	if err != nil {
		return nil, err
	}

	if status == http.StatusUnauthorized {
		refreshed, refreshErr := s.refreshAccessToken(jwt)
		if refreshErr != nil || !refreshed {
			if refreshErr != nil {
				return nil, refreshErr
			}
			return &sessionOrgsResult{JWT: jwt, StatusCode: status, Organizations: []uicfg.SessionOrganization{}}, nil
		}

		s.mu.Lock()
		rawJWT = s.cfg.JWT
		s.mu.Unlock()
		jwt = strings.TrimSpace(rawJWT)
		status, body, err = s.getOrganizationsWithBearer(jwt, apiURL)
		if err != nil {
			return nil, err
		}
	}

	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return &sessionOrgsResult{JWT: jwt, StatusCode: status, Organizations: []uicfg.SessionOrganization{}}, nil
	}

	var payload organizationsListResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode organizations list: %w", err)
	}

	result := &sessionOrgsResult{
		JWT:           jwt,
		StatusCode:    status,
		Organizations: make([]uicfg.SessionOrganization, 0, len(payload.Organizations)),
	}
	for _, org := range payload.Organizations {
		result.Organizations = append(result.Organizations, uicfg.SessionOrganization{
			ID:   org.ID,
			Name: strings.TrimSpace(org.Name),
			Role: strings.TrimSpace(org.RoleName),
		})
	}

	return result, nil
}

func (s *connectorManagerServer) getOrganizationsWithBearer(jwt string, apiURL string) (int, []byte, error) {
	url := network.ReviewNormalizedAPIURL(apiURL, "/api/v1/organizations")
	resp, err := s.client.Do(http.MethodGet, url, nil, map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(jwt),
	})
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("failed to fetch organizations: %w", err)
	}
	return resp.StatusCode, resp.Body, nil
}

func (s *connectorManagerServer) handleSetOrgContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	orgID, parseErr := parseOrgIDFromBody(body)
	if parseErr != nil {
		writeJSONError(w, http.StatusBadRequest, parseErr.Error())
		return
	}

	orgsResult, orgsErr := s.fetchSessionOrganizations()
	if orgsErr != nil {
		writeJSONError(w, http.StatusBadGateway, orgsErr.Error())
		return
	}
	if orgsResult == nil {
		writeJSONError(w, http.StatusBadGateway, "failed to fetch organizations for current session")
		return
	}
	if orgsResult.StatusCode == http.StatusUnauthorized {
		writeJSONError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	if orgsResult.StatusCode < http.StatusOK || orgsResult.StatusCode >= http.StatusMultipleChoices {
		writeJSONError(w, orgsResult.StatusCode, "failed to fetch organizations for current session")
		return
	}

	var selected *uicfg.SessionOrganization
	for i := range orgsResult.Organizations {
		if orgsResult.Organizations[i].ID == orgID {
			selected = &orgsResult.Organizations[i]
			break
		}
	}
	if selected == nil {
		writeJSONError(w, http.StatusForbidden, "organization not available for current user")
		return
	}

	selectedOrgName := strings.TrimSpace(selected.Name)
	orgIDValue := strconv.FormatInt(orgID, 10)

	s.mu.Lock()
	s.cfg.OrgID = orgIDValue
	s.cfg.OrgName = selectedOrgName
	configPath := s.cfg.ConfigPath
	jwt := s.cfg.JWT
	apiURL := s.cfg.APIURL
	s.mu.Unlock()

	var newAPIKey string
	if strings.TrimSpace(jwt) != "" {
		key, keyErr := s.createAPIKeyWithJWT(apiURL, orgIDValue, jwt)
		if keyErr != nil {
			fmt.Printf("Warning: failed to proactively generate new API key for org context switch: %v\n", keyErr)
		} else {
			newAPIKey = key
		}
	}

	if err := persistOrgContextToConfig(configPath, orgIDValue, selectedOrgName, newAPIKey); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to persist organization context: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"org_id":   orgIDValue,
		"org_name": selectedOrgName,
		"message":  "organization context updated",
	})
}

func parseOrgIDFromBody(body []byte) (int64, error) {
	var payload struct {
		OrgID json.RawMessage `json:"org_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("invalid request body")
	}
	rawOrgID := strings.TrimSpace(string(payload.OrgID))
	if rawOrgID == "" || rawOrgID == "null" {
		return 0, fmt.Errorf("org_id is required")
	}

	var orgIDString string
	if err := json.Unmarshal(payload.OrgID, &orgIDString); err == nil {
		trimmed := strings.TrimSpace(orgIDString)
		if trimmed == "" {
			return 0, fmt.Errorf("org_id is required")
		}
		parsed, parseErr := strconv.ParseInt(trimmed, 10, 64)
		if parseErr != nil || parsed <= 0 {
			return 0, fmt.Errorf("org_id must be a positive integer")
		}
		return parsed, nil
	}

	var orgIDNumber json.Number
	if err := json.Unmarshal(payload.OrgID, &orgIDNumber); err == nil {
		parsed, parseErr := strconv.ParseInt(strings.TrimSpace(orgIDNumber.String()), 10, 64)
		if parseErr != nil || parsed <= 0 {
			return 0, fmt.Errorf("org_id must be a positive integer")
		}
		return parsed, nil
	}

	return 0, fmt.Errorf("org_id must be a string or number")
}

type createAPIKeyRuntimeRequest struct {
	Label string `json:"label"`
}

type createAPIKeyRuntimeResponse struct {
	PlainKey string `json:"plain_key"`
}

func (s *connectorManagerServer) createAPIKeyWithJWT(apiURL, orgID, jwtToken string) (string, error) {
	if strings.TrimSpace(jwtToken) == "" {
		return "", fmt.Errorf("missing jwt token")
	}

	payload := createAPIKeyRuntimeRequest{Label: "LRC CLI Key"}
	resp, err := network.SetupCreateAPIKey(s.client, apiURL, orgID, payload, jwtToken)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create API key returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}

	var res createAPIKeyRuntimeResponse
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return "", fmt.Errorf("failed to parse create key response: %w", err)
	}
	if strings.TrimSpace(res.PlainKey) == "" {
		return "", fmt.Errorf("create key response missing plain_key")
	}

	return res.PlainKey, nil
}
