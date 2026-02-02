package xlog

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mappu/miqt/qt6"
	"scouter.client.qt/cache"
	"scouter.client.qt/net"
	"scouter.client.qt/protocol/pack"
)

// Color definitions for profile text
var (
	colorDefault = qt6.NewQColor3(220, 220, 220) // light gray
	colorStep    = qt6.NewQColor3(130, 130, 130)  // dim gray for step number
	colorMethod  = qt6.NewQColor3(180, 180, 180)  // gray for method
	colorSQL     = qt6.NewQColor3(80, 160, 240)   // blue for SQL
	colorAPI     = qt6.NewQColor3(80, 200, 120)    // green for API
	colorSocket  = qt6.NewQColor3(180, 120, 230)   // purple for socket
	colorMessage = qt6.NewQColor3(200, 200, 160)   // yellow-ish for message
	colorError   = qt6.NewQColor3(240, 70, 70)     // red for errors
	colorThread  = qt6.NewQColor3(220, 180, 80)    // orange for thread
	colorElapsed = qt6.NewQColor3(160, 160, 160)   // gray for elapsed
	colorParam   = qt6.NewQColor3(140, 180, 140)   // dim green for params
	colorSum     = qt6.NewQColor3(170, 170, 130)   // dim for summary
	colorControl = qt6.NewQColor3(200, 100, 100)   // reddish for control
)

// Column widths matching Java ProfileText.java format:
//
//	    p#      #          TIME         T-GAP   CPU          CONTENTS
//	    -    [000000] 22:23:28.840        0      0  content here
//	[000071] [000072] 22:23:28.872        0      0   indented content
const (
	colParent = 9  // "    -    " or "[000NNN] "
	colIndex  = 9  // "[000NNN] "
	colTime   = 13 // "HH:mm:ss.SSS "
	colTGap   = 9  // "%8d "
	colCPU    = 7  // "%6d "
)

// Total header width before content (for continuation line indent)
const headerWidth = colParent + colIndex + colTime + colTGap + colCPU

// ProfileView displays XLog profile details in a text view
type ProfileView struct {
	widget    *qt6.QWidget
	textEdit  *qt6.QTextEdit
	textCache *cache.TextCache
	proxy     *net.Proxy

	// Current profile
	profile     *pack.XLogProfilePack
	txID        int64
	startTimeMs int64 // transaction start time (epoch ms)
}

// NewProfileView creates a new profile detail view
func NewProfileView(parent *qt6.QWidget, proxy *net.Proxy) *ProfileView {
	v := &ProfileView{
		widget:    qt6.NewQWidget(parent),
		textCache: cache.GetTextCache(),
		proxy:     proxy,
	}

	// Layout
	layout := qt6.NewQVBoxLayout(v.widget)
	layout.SetContentsMargins(0, 0, 0, 0)
	layout.SetSpacing(4)

	// Toolbar
	toolbar := v.createToolbar()
	layout.AddLayout(toolbar.QLayout)

	// Text edit (read-only, monospace, no line wrap for horizontal scrolling)
	v.textEdit = qt6.NewQTextEdit2()
	v.textEdit.SetReadOnly(true)
	v.textEdit.SetLineWrapMode(qt6.QTextEdit__NoWrap)
	v.textEdit.SetFontFamily("Menlo")

	// Dark background
	palette := v.textEdit.Palette()
	palette.SetColor2(qt6.QPalette__Base, qt6.NewQColor3(30, 30, 30))
	palette.SetColor2(qt6.QPalette__Text, qt6.NewQColor3(220, 220, 220))
	v.textEdit.SetPalette(palette)

	layout.AddWidget(v.textEdit.QWidget)

	v.widget.SetMinimumWidth(600)
	v.widget.SetMinimumHeight(400)

	return v
}

// createToolbar creates the toolbar with action buttons
func (v *ProfileView) createToolbar() *qt6.QHBoxLayout {
	toolbar := qt6.NewQHBoxLayout2()

	// Refresh button
	refreshBtn := qt6.NewQPushButton3("Refresh")
	refreshBtn.OnClicked(func() {
		if v.txID != 0 {
			v.LoadProfileWithTime(v.txID, v.startTimeMs)
		}
	})
	toolbar.AddWidget(refreshBtn.QWidget)

	// Copy button
	copyBtn := qt6.NewQPushButton3("Copy All")
	copyBtn.OnClicked(func() {
		clipboard := qt6.QGuiApplication_Clipboard()
		clipboard.SetText(v.textEdit.ToPlainText())
	})
	toolbar.AddWidget(copyBtn.QWidget)

	toolbar.AddStretch()

	return toolbar
}

// QWidget returns the underlying Qt widget
func (v *ProfileView) QWidget() *qt6.QWidget {
	return v.widget
}

// LoadProfile loads and displays a profile (without absolute time info)
func (v *ProfileView) LoadProfile(txID int64) {
	v.LoadProfileWithTime(txID, 0)
}

// LoadProfileWithTime loads and displays a profile with transaction start time
func (v *ProfileView) LoadProfileWithTime(txID int64, startTimeMs int64) {
	v.txID = txID
	v.startTimeMs = startTimeMs
	v.textEdit.Clear()

	if v.proxy == nil {
		v.appendColored("No proxy connection available", colorError)
		return
	}

	// Fetch profile from server
	profile, err := v.proxy.GetXLogProfile(txID)
	if err != nil {
		v.appendColored(fmt.Sprintf("Failed to load profile: %v", err), colorError)
		return
	}

	if profile == nil {
		v.appendColored("Profile not found", colorError)
		return
	}

	v.profile = profile
	v.renderProfile()
}

// Profile font size in points
const profileFontSize = 12

// newCharFormat creates a QTextCharFormat with the given foreground color and monospace font
func newCharFormat(color *qt6.QColor) *qt6.QTextCharFormat {
	f := qt6.NewQTextCharFormat()
	f.SetForeground(qt6.NewQBrush3(color))
	font := qt6.NewQFont2("Menlo")
	font.SetStyleHint(qt6.QFont__Monospace)
	font.SetPointSize(profileFontSize)
	f.SetFont(font)
	return f
}

// appendColored appends colored text (simple helper for single-line messages)
func (v *ProfileView) appendColored(text string, color *qt6.QColor) {
	cursor := v.textEdit.TextCursor()
	f := newCharFormat(color)
	cursor.InsertText2(text, f)
}

// getBaseStep extracts the BaseStep from a Step, or nil for summary/control steps
func getBaseStep(step pack.Step) *pack.BaseStep {
	switch s := step.(type) {
	case *pack.MethodStep:
		return &s.BaseStep
	case *pack.Method2Step:
		return &s.BaseStep
	case *pack.SqlStep:
		return &s.BaseStep
	case *pack.SqlStep2:
		return &s.BaseStep
	case *pack.SqlStep3:
		return &s.BaseStep
	case *pack.MessageStep:
		return &s.BaseStep
	case *pack.HashedMessageStep:
		return &s.BaseStep
	case *pack.SocketStep:
		return &s.BaseStep
	case *pack.ApiCallStep:
		return &s.BaseStep
	case *pack.ApiCallStep2:
		return &s.BaseStep
	case *pack.ThreadSubmitStep:
		return &s.BaseStep
	case *pack.DispatchStep:
		return &s.BaseStep
	case *pack.DumpStep:
		return &s.BaseStep
	case *pack.ThreadCallPossibleStep:
		return &s.BaseStep
	case *pack.ParameterizedMessageStep:
		return &s.BaseStep
	default:
		return nil
	}
}

// stepElapsed returns the elapsed time of a step
func stepElapsed(step pack.Step) int32 {
	switch s := step.(type) {
	case *pack.MethodStep:
		return s.Elapsed
	case *pack.Method2Step:
		return s.Elapsed
	case *pack.SqlStep:
		return s.Elapsed
	case *pack.SqlStep2:
		return s.Elapsed
	case *pack.SqlStep3:
		return s.Elapsed
	case *pack.SocketStep:
		return s.Elapsed
	case *pack.ApiCallStep:
		return s.Elapsed
	case *pack.ApiCallStep2:
		return s.Elapsed
	case *pack.ThreadSubmitStep:
		return s.Elapsed
	case *pack.DispatchStep:
		return s.Elapsed
	case *pack.ThreadCallPossibleStep:
		return s.Elapsed
	case *pack.ParameterizedMessageStep:
		return s.Elapsed
	default:
		return 0
	}
}

// renderProfile renders the profile data matching Java ProfileText.java format
func (v *ProfileView) renderProfile() {
	if v.profile == nil || len(v.profile.Steps) == 0 {
		v.appendColored("No profile data", colorStep)
		return
	}

	cursor := v.textEdit.TextCursor()
	stepFmt := newCharFormat(colorStep)

	// Separator line
	separator := strings.Repeat("-", 90)
	cursor.InsertText2(separator, stepFmt)
	cursor.InsertText("\n")

	// Column header
	cursor.InsertText2("    p#      #          TIME         T-GAP   CPU          CONTENTS", stepFmt)
	cursor.InsertText("\n")
	cursor.InsertText2(separator, stepFmt)
	cursor.InsertText("\n")

	// Sort steps by index (matching Java's SortUtil.sort which uses Step.getOrder() == index)
	sort.Slice(v.profile.Steps, func(i, j int) bool {
		bi := getBaseStep(v.profile.Steps[i])
		bj := getBaseStep(v.profile.Steps[j])
		var ii, ij int32
		if bi != nil {
			ii = bi.Index
		}
		if bj != nil {
			ij = bj.Index
		}
		return ii < ij
	})

	// "start transaction" marker
	v.writeMarkerLine(cursor, 0, "start transaction")

	// Build indent map and render steps
	indent := make(map[int32]int)
	var prevStartTime int32

	for i, step := range v.profile.Steps {
		cursor.InsertText("\n")

		bs := getBaseStep(step)
		indentLevel := 0
		var tgap int32
		var cpu int32

		if bs != nil {
			// Indent level from parent-child hierarchy
			if parentLevel, ok := indent[bs.Parent]; ok {
				indentLevel = parentLevel + 1
			}
			indent[bs.Index] = indentLevel

			// T-GAP: time difference from previous step
			tgap = bs.StartTime - prevStartTime
			cpu = bs.StartCPU
			prevStartTime = bs.StartTime
		}

		v.renderStep(cursor, i, step, bs, indentLevel, tgap, cpu)
	}

	// "end of transaction" marker
	cursor.InsertText("\n")
	if len(v.profile.Steps) > 0 {
		lastStep := v.profile.Steps[len(v.profile.Steps)-1]
		if bs := getBaseStep(lastStep); bs != nil {
			elapsed := stepElapsed(lastStep)
			v.writeMarkerLine(cursor, bs.StartTime+elapsed, "end of transaction")
		} else {
			v.writeMarkerLine(cursor, prevStartTime, "end of transaction")
		}
	}

	// Final separator
	cursor.InsertText("\n")
	cursor.InsertText2(separator, stepFmt)

	// Move cursor to top
	v.textEdit.MoveCursor(qt6.QTextCursor__Start)
}

// writeMarkerLine writes [******] marker line (start/end transaction)
func (v *ProfileView) writeMarkerLine(cursor *qt6.QTextCursor, startTimeOffset int32, message string) {
	stepFmt := newCharFormat(colorStep)
	msgFmt := newCharFormat(colorMessage)

	// 9-char empty parent column
	cursor.InsertText2("         ", stepFmt)
	cursor.InsertText2("[******] ", stepFmt)
	cursor.InsertText2(v.formatAbsTime(startTimeOffset), stepFmt)
	cursor.InsertText2(fmt.Sprintf("%8d", 0), stepFmt)
	cursor.InsertText2(fmt.Sprintf("%6d", 0), stepFmt)
	cursor.InsertText2("  ", stepFmt)
	cursor.InsertText2(message+" ", msgFmt)
}

// formatAbsTime formats absolute time from step's StartTime offset
func (v *ProfileView) formatAbsTime(offsetMs int32) string {
	if v.startTimeMs > 0 {
		t := time.UnixMilli(v.startTimeMs + int64(offsetMs))
		return t.Format("15:04:05.000") + " "
	}
	// Fallback: show relative offset
	return fmt.Sprintf("%12d ", offsetMs)
}

// writeStepPrefix writes the parent + index columns
// Java format: "    -    [000NNN] " for root (parent==-1), "[PARENT] [000NNN] " for child
func (v *ProfileView) writeStepPrefix(cursor *qt6.QTextCursor, bs *pack.BaseStep, stepIdx int) {
	stepFmt := newCharFormat(colorStep)

	if bs == nil || bs.Parent == -1 {
		// Root step: "    -    "
		cursor.InsertText2("    -    ", stepFmt)
	} else {
		// Child step: "[PARENT] "
		cursor.InsertText2(fmt.Sprintf("[%06d] ", bs.Parent), stepFmt)
	}

	// Step index: "[000NNN] "
	if bs != nil {
		cursor.InsertText2(fmt.Sprintf("[%06d] ", bs.Index), stepFmt)
	} else {
		cursor.InsertText2(fmt.Sprintf("[%06d] ", stepIdx), stepFmt)
	}
}

// writeStepTimeColumns writes TIME, T-GAP, CPU columns
func (v *ProfileView) writeStepTimeColumns(cursor *qt6.QTextCursor, bs *pack.BaseStep, tgap, cpu int32) {
	stepFmt := newCharFormat(colorStep)

	if bs != nil {
		cursor.InsertText2(v.formatAbsTime(bs.StartTime), stepFmt)
	} else {
		cursor.InsertText2(strings.Repeat(" ", colTime), stepFmt)
	}

	cursor.InsertText2(fmt.Sprintf("%8d", tgap), stepFmt)
	cursor.InsertText2(fmt.Sprintf("%6d", cpu), stepFmt)
}

// writeContentIndent writes the spacing before content (2 base + indent level)
func writeContentIndent(cursor *qt6.QTextCursor, indentLevel int) {
	stepFmt := newCharFormat(colorStep)
	cursor.InsertText2("  "+strings.Repeat(" ", indentLevel), stepFmt)
}

// writeContinuationIndent writes a new line with proper indentation for sub-info (params, errors)
func writeContinuationIndent(cursor *qt6.QTextCursor, indentLevel int) {
	cursor.InsertText("\n")
	stepFmt := newCharFormat(colorStep)
	cursor.InsertText2(strings.Repeat(" ", headerWidth+2+indentLevel), stepFmt)
}

// renderStep renders a single profile step in Java format
func (v *ProfileView) renderStep(cursor *qt6.QTextCursor, stepIdx int, step pack.Step, bs *pack.BaseStep, indent int, tgap, cpu int32) {
	// Write prefix columns: parent, index, time, tgap, cpu
	v.writeStepPrefix(cursor, bs, stepIdx)
	v.writeStepTimeColumns(cursor, bs, tgap, cpu)
	writeContentIndent(cursor, indent)

	// Write content based on step type
	switch s := step.(type) {
	case *pack.MethodStep:
		v.renderMethodContent(cursor, s, indent)
	case *pack.Method2Step:
		v.renderMethod2Content(cursor, s, indent)
	case *pack.SqlStep:
		v.renderSqlContent(cursor, s, 0, 0, indent)
	case *pack.SqlStep2:
		v.renderSqlContent(cursor, &s.SqlStep, s.XType, 0, indent)
	case *pack.SqlStep3:
		v.renderSqlContent(cursor, &s.SqlStep, s.XType, s.Updated, indent)
	case *pack.ApiCallStep:
		v.renderApiCallContent(cursor, s, indent)
	case *pack.ApiCallStep2:
		v.renderApiCallContent(cursor, &s.ApiCallStep, indent)
	case *pack.SocketStep:
		v.renderSocketContent(cursor, s, indent)
	case *pack.MessageStep:
		cursor.InsertText2(s.Message, newCharFormat(colorMessage))
	case *pack.HashedMessageStep:
		v.renderHashedMessageContent(cursor, s)
	case *pack.MethodSum:
		v.renderMethodSumContent(cursor, s)
	case *pack.SqlSum:
		v.renderSqlSumContent(cursor, s)
	case *pack.ApiCallSum:
		v.renderApiCallSumContent(cursor, s)
	case *pack.SocketSum:
		v.renderSocketSumContent(cursor, s)
	case *pack.ThreadSubmitStep:
		v.renderThreadSubmitContent(cursor, s)
	case *pack.DispatchStep:
		v.renderDispatchContent(cursor, s, indent)
	case *pack.DumpStep:
		cursor.InsertText2(fmt.Sprintf("Thread: %s [%s]", s.ThreadName, s.ThreadState), newCharFormat(colorThread))
	case *pack.ParameterizedMessageStep:
		v.renderParameterizedMsgContent(cursor, s, indent)
	case *pack.ThreadCallPossibleStep:
		v.renderThreadCallContent(cursor, s)
	case *pack.StepControl:
		cursor.InsertText2(s.Message, newCharFormat(colorControl))
	default:
		cursor.InsertText2(fmt.Sprintf("type=%d", step.StepType()), newCharFormat(colorDefault))
	}
}

func (v *ProfileView) renderMethodContent(cursor *qt6.QTextCursor, step *pack.MethodStep, indent int) {
	methodName := v.textCache.GetMethod(step.Hash)
	if methodName == "" {
		methodName = fmt.Sprintf("method#%d", step.Hash)
	}

	cursor.InsertText2(fmt.Sprintf("%s [%dms]", methodName, step.Elapsed), newCharFormat(colorMethod))
}

func (v *ProfileView) renderMethod2Content(cursor *qt6.QTextCursor, step *pack.Method2Step, indent int) {
	v.renderMethodContent(cursor, &step.MethodStep, indent)

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderSqlContent(cursor *qt6.QTextCursor, step *pack.SqlStep, xtype byte, updated int32, indent int) {
	sqlText := v.textCache.GetSQL(step.Hash)
	if sqlText == "" {
		sqlText = fmt.Sprintf("sql#%d", step.Hash)
	}

	prefix := sqlXTypePrefix(xtype)
	cursor.InsertText2(prefix+formatSQL(sqlText), newCharFormat(colorSQL))

	// Param on continuation line
	if step.Param != "" {
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2(fmt.Sprintf("[%s] %d ms", step.Param, step.Elapsed), newCharFormat(colorParam))
	} else {
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2(fmt.Sprintf("[%d] %d ms", 0, step.Elapsed), newCharFormat(colorParam))
	}

	// Updated/fetch info for SqlStep3
	if updated != 0 {
		writeContinuationIndent(cursor, indent)
		if updated == -1 {
			cursor.InsertText2("RESULT-SET", newCharFormat(colorMessage))
		} else if updated >= 0 {
			cursor.InsertText2(fmt.Sprintf("UPDATED %d rows", updated), newCharFormat(colorMessage))
		}
	}

	// Error
	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func sqlXTypePrefix(xtype byte) string {
	switch xtype & 0x0f {
	case 0x01:
		return "PRE> "
	case 0x02:
		return "DYN> "
	default:
		return "STM> "
	}
}

func (v *ProfileView) renderApiCallContent(cursor *qt6.QTextCursor, step *pack.ApiCallStep, indent int) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("api#%d", step.Hash)
	}

	cursor.InsertText2(fmt.Sprintf("%s [%dms]", apiURL, step.Elapsed), newCharFormat(colorAPI))

	if step.Address != "" {
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ADDR "+step.Address, newCharFormat(colorParam))
	}

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderSocketContent(cursor *qt6.QTextCursor, step *pack.SocketStep, indent int) {
	ipAddr := formatIPAddr(step.IPAddr)
	cursor.InsertText2(fmt.Sprintf("%s:%d [%dms]", ipAddr, step.Port, step.Elapsed), newCharFormat(colorSocket))

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderHashedMessageContent(cursor *qt6.QTextCursor, step *pack.HashedMessageStep) {
	message := v.textCache.GetMessage(step.Hash)
	if message == "" {
		message = fmt.Sprintf("msg#%d", step.Hash)
	}
	// Java format: "MESSAGE #VALUE TIME ms" (when time != -1)
	if step.Time != -1 {
		message = fmt.Sprintf("%s #%d %d ms", message, step.Value, step.Time)
	}
	cursor.InsertText2(message, newCharFormat(colorMessage))
}

func (v *ProfileView) renderMethodSumContent(cursor *qt6.QTextCursor, step *pack.MethodSum) {
	methodName := v.textCache.GetMethod(step.Hash)
	if methodName == "" {
		methodName = fmt.Sprintf("method#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s (x%d, %dms)", methodName, step.Count, step.Elapsed), newCharFormat(colorSum))
}

func (v *ProfileView) renderSqlSumContent(cursor *qt6.QTextCursor, step *pack.SqlSum) {
	sqlText := v.textCache.GetSQL(step.Hash)
	if sqlText == "" {
		sqlText = fmt.Sprintf("sql#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s (x%d, %dms)", formatSQL(sqlText), step.Count, step.Elapsed), newCharFormat(colorSum))
}

func (v *ProfileView) renderApiCallSumContent(cursor *qt6.QTextCursor, step *pack.ApiCallSum) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("api#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s (x%d, %dms)", apiURL, step.Count, step.Elapsed), newCharFormat(colorSum))
}

func (v *ProfileView) renderSocketSumContent(cursor *qt6.QTextCursor, step *pack.SocketSum) {
	ipAddr := formatIPAddr(step.IPAddr)
	cursor.InsertText2(fmt.Sprintf("%s:%d (x%d, %dms)", ipAddr, step.Port, step.Count, step.Elapsed), newCharFormat(colorSum))
}

func (v *ProfileView) renderThreadSubmitContent(cursor *qt6.QTextCursor, step *pack.ThreadSubmitStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("thread#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s [%dms]", apiURL, step.Elapsed), newCharFormat(colorThread))
}

func (v *ProfileView) renderDispatchContent(cursor *qt6.QTextCursor, step *pack.DispatchStep, indent int) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("dispatch#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s [%dms]", apiURL, step.Elapsed), newCharFormat(colorThread))

	if step.Address != "" {
		writeContinuationIndent(cursor, indent)
		cursor.InsertText2("ADDR "+step.Address, newCharFormat(colorParam))
	}
}

func (v *ProfileView) renderParameterizedMsgContent(cursor *qt6.QTextCursor, step *pack.ParameterizedMessageStep, indent int) {
	messageFormat := v.textCache.GetMessage(step.Hash)
	if messageFormat == "" {
		messageFormat = fmt.Sprintf("msg#%d", step.Hash)
	}

	// Substitute %s placeholders with params (split by ETX char 3), matching Java's String.format()
	message := buildParameterizedMessage(messageFormat, step.ParamString)

	color := parameterizedMsgColor(step.Level)
	if step.Elapsed >= 0 {
		cursor.InsertText2(fmt.Sprintf("%s [%d ms]", message, step.Elapsed), newCharFormat(color))
	} else {
		cursor.InsertText2(message, newCharFormat(color))
	}
}

// buildParameterizedMessage substitutes %s placeholders in format string with params
func buildParameterizedMessage(messageFormat string, paramString string) string {
	if paramString == "" {
		return messageFormat
	}
	// Java splits by ETX (char 3)
	params := strings.Split(paramString, string(rune(3)))
	if len(params) == 0 {
		return messageFormat
	}
	// Replace %s placeholders sequentially (matching Java's String.format behavior)
	result := messageFormat
	for _, p := range params {
		idx := strings.Index(result, "%s")
		if idx < 0 {
			break
		}
		result = result[:idx] + p + result[idx+2:]
	}
	return result
}

// parameterizedMsgColor returns color based on message level (DEBUG=0, INFO=1, WARN=2, ERROR=3, FATAL=4)
func parameterizedMsgColor(level int32) *qt6.QColor {
	switch level {
	case 2: // WARN
		return qt6.NewQColor3(200, 160, 50) // dark orange
	case 3: // ERROR
		return qt6.NewQColor3(240, 100, 100) // light red
	case 4: // FATAL
		return qt6.NewQColor3(240, 70, 70) // red
	default: // DEBUG=0, INFO=1
		return colorMessage
	}
}

func (v *ProfileView) renderThreadCallContent(cursor *qt6.QTextCursor, step *pack.ThreadCallPossibleStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("thread#%d", step.Hash)
	}
	cursor.InsertText2(fmt.Sprintf("%s [%dms]", apiURL, step.Elapsed), newCharFormat(colorThread))
}

// formatSQL formats SQL text with basic whitespace cleanup
func formatSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}

// formatIPAddr formats IP address bytes to string
func formatIPAddr(ip []byte) string {
	if len(ip) < 4 {
		return "0.0.0.0"
	}
	return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
}
