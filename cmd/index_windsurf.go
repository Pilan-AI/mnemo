// index_windsurf.go indexes Windsurf IDE sessions from its SQLite state databases.
// Windsurf stores cascade conversations as unencrypted base64-encoded protobuf
// inside the codeium.windsurf JSON blob in globalStorage/state.vscdb.
// The .pb files in ~/.codeium/windsurf/cascade/ are encrypted at rest and are
// NOT parsed by this adapter. The SQLite cache is the authoritative live source.
package cmd

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Pilan-AI/mnemo/internal/db"
)

func indexWindsurf(home string) (int, int) {
	totalSessions := 0
	totalMessages := 0

	wsPath := filepath.Join(appSupportDir(home, "Windsurf"), "User", "workspaceStorage")
	if pathExists(wsPath) {
		s, m := indexWindsurfWorkspaceChat(wsPath)
		totalSessions += s
		totalMessages += m
	}

	globalPath := filepath.Join(appSupportDir(home, "Windsurf"), "User", "globalStorage", "state.vscdb")
	if pathExists(globalPath) && !isSourceDBUnchanged(globalPath, "windsurf") {
		s, m := indexWindsurfTrajectoryCache(globalPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

// ---------- Workspace aichat ----------

func indexWindsurfWorkspaceChat(workspaceStoragePath string) (int, int) {
	entries, err := os.ReadDir(workspaceStoragePath)
	if err != nil {
		return 0, 0
	}
	totalSessions := 0
	totalMessages := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dbPath := filepath.Join(workspaceStoragePath, entry.Name(), "state.vscdb")
		if !pathExists(dbPath) {
			continue
		}
		info, err := os.Stat(dbPath)
		if err != nil || skipOldFile(info) || isSessionUnchanged(entry.Name(), info.ModTime()) {
			continue
		}
		s, m := indexWindsurfWorkspace(dbPath, entry.Name())
		totalSessions += s
		totalMessages += m
	}
	return totalSessions, totalMessages
}

func indexWindsurfWorkspace(dbPath, workspaceID string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = sqliteDB.Close() }()

	keys := []string{
		"workbench.panel.aichat.view.aichat.chatdata",
		"aiChat.chatdata",
		"chat.data",
		"cascade.chatdata",
	}
	var blob string
	for _, k := range keys {
		row := sqliteDB.QueryRow("SELECT value FROM ItemTable WHERE key = ?", k)
		if err := row.Scan(&blob); err == nil && blob != "" {
			break
		}
	}
	if blob == "" {
		return 0, 0
	}

	var chatData struct {
		Tabs []struct {
			TabID     string `json:"tabId"`
			ChatTitle string `json:"chatTitle"`
			Bubbles   []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				RawText  string `json:"rawText"`
				InitText string `json:"initText"`
			} `json:"bubbles"`
		} `json:"tabs"`
	}
	if err := json.Unmarshal([]byte(blob), &chatData); err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0
	for _, tab := range chatData.Tabs {
		if len(tab.Bubbles) == 0 || tab.TabID == "" {
			continue
		}
		projectName := tab.ChatTitle
		if projectName == "" {
			projectName = "windsurf-" + truncate(workspaceID, 8)
		}
		s, m := indexWindsurfTab(tab.TabID, projectName, dbPath, tab.Bubbles)
		totalSessions += s
		totalMessages += m
	}
	return totalSessions, totalMessages
}

func indexWindsurfTab(sessionID, projectName, dbPath string, bubbles []struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	RawText  string `json:"rawText"`
	InitText string `json:"initText"`
}) (int, int) {
	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, sessionID); err != nil {
		indexErrors++
		return 0, 0
	}

	var firstUserMsg string
	msgCount := 0

	for _, b := range bubbles {
		content := b.Text
		if content == "" {
			content = b.RawText
		}
		if content == "" {
			content = b.InitText
		}
		if content == "" {
			continue
		}

		role := "user"
		if b.Type == "2" || b.Type == "ai" || b.Type == "assistant" {
			role = "assistant"
		}

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		if err := db.TxInsertMessage(tx, db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
			Tool:      "windsurf",
		}); err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		if err := db.TxInsertSessionSimple(tx, sessionID, projectName, firstUserMsg, dbPath, "windsurf", msgCount); err != nil {
			indexErrors++
			return 0, msgCount
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, msgCount
		}
		return 1, msgCount
	}
	return 0, 0
}

// ---------- Global trajectory cache ----------

// trajectorySummary holds session metadata from cachedTrajectorySummaries.
type trajectorySummary struct {
	Title string
	Model string
}

// indexWindsurfTrajectoryCache extracts cascade conversations from the
// codeium.windsurf JSON blob in globalStorage state.vscdb.
func indexWindsurfTrajectoryCache(dbPath string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = sqliteDB.Close() }()

	var codeiumJSON string
	row := sqliteDB.QueryRow("SELECT value FROM ItemTable WHERE key = 'codeium.windsurf'")
	if err := row.Scan(&codeiumJSON); err != nil || codeiumJSON == "" {
		return 0, 0
	}

	var codeiumData map[string]json.RawMessage
	if err := json.Unmarshal([]byte(codeiumJSON), &codeiumData); err != nil {
		return 0, 0
	}

	// Build workspace-level metadata map from trajectory summaries.
	// Keyed by workspace ID since summaries and active trajectories share
	// the same workspace ID suffix in their key names.
	wsMetaMap := buildWorkspaceMetaMap(codeiumData)

	indexedIDs := make(map[string]bool)
	totalSessions := 0
	totalMessages := 0

	for k, v := range codeiumData {
		if !strings.HasPrefix(k, "windsurf.state.cachedActiveTrajectory:") {
			continue
		}
		workspaceID := strings.TrimPrefix(k, "windsurf.state.cachedActiveTrajectory:")
		b64, ok := unquoteJSONString(v)
		if !ok {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			continue
		}
		meta := wsMetaMap[workspaceID]
		s, m := indexWindsurfActiveTrajectory(raw, meta, dbPath, indexedIDs)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

// buildWorkspaceMetaMap extracts session titles from cachedTrajectorySummaries,
// keyed by workspace ID (the suffix after the colon in the key name).
func buildWorkspaceMetaMap(codeiumData map[string]json.RawMessage) map[string]trajectorySummary {
	result := make(map[string]trajectorySummary)
	for k, v := range codeiumData {
		if !strings.HasPrefix(k, "windsurf.state.cachedTrajectorySummaries:") {
			continue
		}
		workspaceID := strings.TrimPrefix(k, "windsurf.state.cachedTrajectorySummaries:")
		b64, ok := unquoteJSONString(v)
		if !ok {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			continue
		}
		strs := extractCleanStrings(raw)
		meta := result[workspaceID]
		for _, s := range strs {
			if isUUID(s) {
				continue
			}
			if meta.Model == "" && isModelName(s) {
				meta.Model = s
				continue
			}
			cleaned := strings.TrimLeft(s, " @\"")
			if len(cleaned) > 5 && len(cleaned) < 200 &&
				!isModelName(cleaned) && !isUUID(cleaned) &&
				!strings.HasPrefix(cleaned, "file://") &&
				!strings.HasPrefix(cleaned, "/") {
				if meta.Title == "" {
					meta.Title = cleaned
				}
				if meta.Model != "" {
					break
				}
			}
		}
		result[workspaceID] = meta
	}
	return result
}

func indexWindsurfActiveTrajectory(raw []byte, meta trajectorySummary, dbPath string, indexedIDs map[string]bool) (int, int) {
	sessionID, contentBlob := parseTopLevelTrajectory(raw)
	if sessionID == "" || len(contentBlob) == 0 {
		return 0, 0
	}
	if indexedIDs[sessionID] {
		return 0, 0
	}
	indexedIDs[sessionID] = true

	allStrings := extractCleanStrings(contentBlob)
	if len(allStrings) == 0 {
		return 0, 0
	}

	projectName := "windsurf-cascade"
	modelName := ""
	if meta.Title != "" {
		projectName = meta.Title
	}
	if meta.Model != "" {
		modelName = meta.Model
	}

	messages := classifyMessages(allStrings)

	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, sessionID); err != nil {
		indexErrors++
		return 0, 0
	}

	var firstUserMsg string
	msgCount := 0

	for _, msg := range messages {
		if msg.Content == "" {
			continue
		}
		if msg.Role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.Content, 200)
		}
		if err := db.TxInsertMessage(tx, db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: time.Now(),
			Tool:      "windsurf",
			Model:     modelName,
		}); err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		if err := db.TxInsertSessionSimple(tx, sessionID, projectName, firstUserMsg, dbPath, "windsurf", msgCount); err != nil {
			indexErrors++
			return 0, msgCount
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, msgCount
		}
		return 1, msgCount
	}
	return 0, 0
}

// ---------- Role classification ----------

type classifiedMessage struct {
	Role    string
	Content string
}

// classifyMessages distinguishes user prompts from assistant responses.
func classifyMessages(strs []string) []classifiedMessage {
	var result []classifiedMessage
	lastRole := ""

	for _, s := range strs {
		s = strings.TrimSpace(s)
		if len(s) < 3 {
			continue
		}

		// Pre-filter: drop metadata orphans (model names, UUIDs, file paths,
		// language tokens) before role classification. Control-character leakage
		// is already rejected at the extractCleanStrings level via isCleanString.
		if isModelName(s) || isUUID(s) || isMetadataString(s) {
			continue
		}

		role := classifyRole(s, lastRole)
		if role == "" {
			continue
		}

		// Merge consecutive same-role messages
		if len(result) > 0 && result[len(result)-1].Role == role {
			result[len(result)-1].Content += "\n\n" + s
		} else {
			result = append(result, classifiedMessage{Role: role, Content: s})
		}
		lastRole = role
	}
	return result
}

func classifyRole(s, lastRole string) string {
	if isToolCall(s) {
		return "assistant"
	}
	if len(s) > 800 && hasMarkdownOrCode(s) {
		return "assistant"
	}
	if len(s) < 150 && !hasMarkdownOrCode(s) {
		if lastRole == "assistant" {
			return "user"
		}
	}
	if lastRole == "user" {
		return "assistant"
	}
	return "user"
}

// isMetadataString identifies protobuf-extracted strings that are metadata,
// not conversation content: file paths, bot IDs, language tokens, etc.
func isMetadataString(s string) bool {
	// Pure file paths (short orphans, not narrative content containing paths).
	if (strings.HasPrefix(s, "file://") || strings.HasPrefix(s, "/Users/") || strings.HasPrefix(s, "/home/")) && len(s) < 150 {
		return true
	}
	// Bot/agent IDs.
	if strings.HasPrefix(s, "bot-") || strings.HasPrefix(s, "agent-") {
		return true
	}
	// Short single-word language identifiers without narrative context.
	if len(s) < 20 && !strings.Contains(s, " ") && !strings.Contains(s, ".") {
		lower := strings.ToLower(s)
		for _, l := range []string{"typescript", "javascript", "python", "rust", "go", "java", "ruby", "swift", "kotlin", "html", "css", "json", "yaml", "toml", "bash", "shell", "sql", "markdown"} {
			if lower == l {
				return true
			}
		}
	}
	return false
}

func isToolCall(s string) bool {
	return strings.Contains(s, `"tool_calls"`) ||
		strings.Contains(s, `"function"`) ||
		strings.Contains(s, `"arguments"`) ||
		strings.Contains(s, `"tool_use_id"`) ||
		strings.HasPrefix(s, "functions.")
}

func hasMarkdownOrCode(s string) bool {
	return strings.Contains(s, "```") ||
		strings.Contains(s, "# ") ||
		strings.Contains(s, "| ") ||
		strings.Contains(s, "**") ||
		strings.Contains(s, "- ")
}

// ---------- Protobuf parsing ----------

// parseTopLevelTrajectory extracts session ID and content blob from the
// top-level trajectory protobuf. Field 1 (LD) = UUID, Field 2 (LD) = content.
func parseTopLevelTrajectory(raw []byte) (string, []byte) {
	i := 0
	var sessionID string
	var contentBlob []byte

	for i < len(raw) && (sessionID == "" || contentBlob == nil) {
		tag, n := readVarint(raw[i:])
		if n == 0 {
			break
		}
		i += n

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)

		if fieldNum == 0 || fieldNum > 5 || wireType != 2 {
			if wireType == 0 {
				_, n := readVarint(raw[i:])
				i += n
			} else if wireType == 1 {
				i += 8
			} else if wireType == 5 {
				i += 4
			} else {
				break
			}
			continue
		}

		length, n := readVarint(raw[i:])
		if n == 0 || i+n > len(raw) || length > uint64(len(raw)-i-n) {
			break
		}
		i += n
		payload := raw[i : i+int(length)]
		i += int(length)

		if fieldNum == 1 && utf8.Valid(payload) {
			s := string(payload)
			if isUUID(s) {
				sessionID = s
			}
		} else if fieldNum == 2 {
			contentBlob = payload
		}
	}
	return sessionID, contentBlob
}

// extractCleanStrings recursively extracts UTF-8 strings from protobuf,
// rejecting strings that contain binary noise or protobuf wire bytes.
func extractCleanStrings(raw []byte) []string {
	var result []string
	i := 0

	for i < len(raw) {
		tag, n := readVarint(raw[i:])
		if n == 0 {
			i++
			continue
		}
		i += n

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x07)

		if fieldNum == 0 || fieldNum > 1000 {
			i++
			continue
		}

		switch wireType {
		case 0:
			_, n := readVarint(raw[i:])
			if n == 0 {
				i++
				continue
			}
			i += n

		case 2:
			length, n := readVarint(raw[i:])
			if n == 0 || i+n > len(raw) || length > uint64(len(raw)-i-n) {
				i++
				continue
			}
			i += n
			payload := raw[i : i+int(length)]
			i += int(length)

			if length < 4 || length > 500000 {
				continue
			}

			if utf8.Valid(payload) {
				s := string(payload)
				if isCleanString(s) {
					result = append(result, s)
					continue
				}
			}

			if length > 10 {
				subStrs := extractCleanStrings(payload)
				result = append(result, subStrs...)
			}

		case 1:
			if i+8 > len(raw) {
				i = len(raw)
			} else {
				i += 8
			}

		case 5:
			if i+4 > len(raw) {
				i = len(raw)
			} else {
				i += 4
			}

		default:
			i++
		}
	}
	return result
}

// isCleanString checks if a string is mostly printable text without
// protobuf wire byte contamination or binary noise.
func isCleanString(s string) bool {
	if len(s) < 4 {
		return false
	}

	// HARD REJECT: any control character except \n, \t, \r.
	// Protobuf wire bytes (\x08, \x12, \x1a, etc.) leak into extracted
	// strings and corrupt first_query. The printable-ratio heuristic
	// (90% threshold) is too permissive for long strings with one
	// or two embedded wire bytes.
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b < 32 && b != '\n' && b != '\t' && b != '\r' {
			return false
		}
	}

	printable := 0
	for _, r := range s {
		if (r >= 32 && r < 127) || r == '\n' || r == '\t' || r == '\r' {
			printable++
		} else if r >= 128 && r <= 0xFFFF {
			printable++
		}
	}

	if float64(printable)/float64(utf8.RuneCountInString(s)) < 0.90 {
		return false
	}

	if strings.ContainsRune(s, 0) {
		return false
	}

	if hexLikeRegex.MatchString(s) {
		return false
	}

	if isUUID(s) {
		return false
	}

	return true
}

// readVarint reads a protobuf varint from the buffer.
func readVarint(buf []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i, b := range buf {
		if i > 9 {
			return 0, 0
		}
		val |= uint64(b&0x7f) << shift
		if (b & 0x80) == 0 {
			return val, i + 1
		}
		shift += 7
	}
	return 0, 0
}

// ---------- Utility functions ----------

var (
	hexLikeRegex = regexp.MustCompile(`^[0-9a-fA-F]{20,}$|^[A-Za-z0-9+/]{40,}={0,2}$`)
	uuidRegex    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

func isUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func isModelName(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "claude") ||
		strings.Contains(lower, "gpt-") ||
		strings.Contains(lower, "kimi-") ||
		strings.Contains(lower, "deepseek") ||
		strings.Contains(lower, "llama") ||
		strings.Contains(lower, "gemini") ||
		strings.Contains(lower, "glm-") ||
		strings.HasPrefix(s, "MODEL_")
}

func unquoteJSONString(raw json.RawMessage) (string, bool) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}
