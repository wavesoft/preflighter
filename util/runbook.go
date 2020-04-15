package util

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/imdario/mergo"
	"github.com/lithammer/dedent"
)

type RunbookClient struct {
	client    *http.Client
	baseUrl   string
	authToken string
}

type apiResponse struct {
	Status string          `json:"status"`
	Error  string          `json:"error"`
	Data   json.RawMessage `json:"data"`
}

/**
 * @brief      Create an instance of the Runbook Client
 *
 * @param      baseUrl    The base url
 * @param      authToken  The auth token
 */
func CreateRunbookClient(baseUrl string, authToken string) (*RunbookClient, error) {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: customTransport}

	return &RunbookClient{
		client:    client,
		baseUrl:   baseUrl,
		authToken: authToken,
	}, nil
}

/**
 * @brief      Creates a runbook client with environment configuration.
 */
func CreateRunbookClientWithEnvConfig() (*RunbookClient, error) {
	baseUrl := os.Getenv("RUNBOOK_URL")
	if baseUrl == "" {
		baseUrl = "https://scaletesting-runbook.mesosphere.com"
	}

	authToken := os.Getenv("RUNBOOK_KEY")
	if authToken == "" {
		return nil, fmt.Errorf("Missing Personal Authentication Token in the RUNBOOK_KEY environment variable")
	}

	return CreateRunbookClient(baseUrl, authToken)
}

/**
 * @brief      Perform an API request
 *
 * @param      verb     The HTTP method to use
 * @param      path     The path
 * @param      apiReq   The api request
 * @param      apiResp  The api response
 *
 * @return     Returns the error occurred or nil
 */
func (c *RunbookClient) apiDo(verb string, path string, apiReq interface{}, apiResp interface{}) error {
	var body []byte
	var respBody apiResponse
	var err error

	if apiReq != nil {
		body, err = json.Marshal(apiReq)
		if err != nil {
			return fmt.Errorf("Could not marshal request: %s", err.Error())
		}
	}

	url := fmt.Sprintf("%s%s", c.baseUrl, path)
	req, err := http.NewRequest(verb, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Could compose request: %s", err.Error())
	}

	if c.authToken != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", c.authToken))
	}
	if strings.ToLower(verb) != "get" {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("Could not place request: %s", err.Error())
	}
	defer resp.Body.Close()

	body, _ = ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		return fmt.Errorf("Could not parse response: %s", err.Error())
	}
	if respBody.Status != "ok" {
		return fmt.Errorf("Server replied with error: %s", respBody.Error)
	}

	if apiResp != nil {
		err = json.Unmarshal(respBody.Data, apiResp)
		if err != nil {
			return fmt.Errorf("Could not unmarshal response: %s", err.Error())
		}
	}

	return nil
}

/**
 * @brief      Returns the variables defined in the runbook for the given domain
 *
 * @param      domain  The domain
 *
 * @return     The variables.
 */
func (c *RunbookClient) GetVariables(domain string) (map[string]string, error) {
	var varsResponse struct {
		Value map[string]interface{}
	}

	// Get all the dynamic variables used in the operation
	err := c.apiDo("GET", "/op/vars/global", nil, &varsResponse)
	if err != nil {
		return nil, err
	}

	vars := make(map[string]string)
	for k, v := range varsResponse.Value {
		vars[k] = fmt.Sprintf("%v", v)
	}

	return vars, nil
}

/**
 * @brief      Try to compose a set of commands to invoke by fetching the
 *             instructions from the runbook app.
 *
 * @param      component  The component
 * @param      step       The step
 *
 * @return     Returns
 */
func (c *RunbookClient) ChecklistFromRunbook(step string) (Checklist, error) {
	rxBlock := regexp.MustCompile(`\x60\x60\x60sh([\w\W]*)\x60\x60\x60`)
	type RunbookChecklistItem struct {
		Id     string `json:"id"`
		Title  string `json:"title"`
		Status int    `json:"status"`
	}
	var checklist Checklist = nil
	var checklists []RunbookChecklistItem
	var stepInfo struct {
		Component    string `json:"component"`
		Instructions string `json:"instructions"`
	}

	// Get all the dynamic variables used in the operation
	vars, err := c.GetVariables("global")

	// Get the step info to get the instructions markdown
	err = c.apiDo("GET", fmt.Sprintf("/step/%s", step), nil, &stepInfo)
	if err != nil {
		return nil, err
	}

	// Get component-local variables
	localVars, err := c.GetVariables(stepInfo.Component)
	err = mergo.Merge(&vars, localVars)
	if err != nil {
		return nil, err
	}

	// Then collect the tokenized step details
	err = c.apiDo("GET", fmt.Sprintf("/step/%s/checklist", step), nil, &checklists)
	if err != nil {
		return nil, err
	}

	// Get all the items from the markdown in order to preserve the order
	rx := regexp.MustCompile(`{!([^}]+)}`)
	ids := rx.FindAllStringSubmatch(stepInfo.Instructions, -1)

	var orderedChecklists []*RunbookChecklistItem
	var found *RunbookChecklistItem = nil
	for _, match := range ids {
		found = nil
		for _, item := range checklists {
			// Don't include completed and skipped items
			if item.Status == 1 || item.Status == 3 {
				continue
			}
			if item.Id == match[1] {
				found = &item
				break
			}
		}

		if found != nil {
			orderedChecklists = append(orderedChecklists, found)
		}
	}

	// And for each step extract the respective markdown checklist item text
	// from the instructions markdown
	for _, item := range orderedChecklists {

		rx := regexp.MustCompile(fmt.Sprintf(`{!%s}([\w\W]+?)([\r\n]\s*\*|$)`, item.Id))
		parts := rx.FindStringSubmatch(stepInfo.Instructions)
		if parts == nil {
			continue
		}

		// On each markdown block, try to locate a shell script block
		parts = rxBlock.FindStringSubmatch(parts[1])
		if parts == nil {
			continue
		}

		// Replace all variables
		script := dedent.Dedent(parts[1])
		for key, value := range vars {
			script = strings.ReplaceAll(script, fmt.Sprintf("{{%s}}", key), value)
		}

		// Collect checklist item
		checklist = append(checklist, ChecklistItem{
			Title:       item.Title,
			Script:      script,
			RunbookID:   item.Id,
			RunbookStep: step,
		})
	}

	return checklist, nil
}

/**
 * @brief      Update the checklist item with the given status
 *
 * @param      id       The identifier
 * @param      status   The state
 * @param      reason   The message
 *
 * @return     Returns the failure if it happened
 */
func (c *RunbookClient) ChecklistItemUpdate(stepId string, itemId string, status int, reason string) error {
	var updateItemStatus struct {
		Status int    `json:"status"`
		Reason string `json:"reason,omitempty"`
	}

	updateItemStatus.Status = status
	updateItemStatus.Reason = reason

	return c.apiDo("PATCH", fmt.Sprintf("/step/%s/checklist/%s", stepId, itemId), updateItemStatus, nil)
}
