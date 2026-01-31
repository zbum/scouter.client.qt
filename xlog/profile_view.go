package xlog

import (
	"fmt"
	"strings"

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

// ProfileView displays XLog profile details in a text view
type ProfileView struct {
	widget    *qt6.QWidget
	textEdit  *qt6.QTextEdit
	textCache *cache.TextCache
	proxy     *net.Proxy

	// Current profile
	profile *pack.XLogProfilePack
	txID    int64
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
			v.LoadProfile(v.txID)
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

// LoadProfile loads and displays a profile for the given transaction ID
func (v *ProfileView) LoadProfile(txID int64) {
	v.txID = txID
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

// renderProfile renders the profile data
func (v *ProfileView) renderProfile() {
	if v.profile == nil || len(v.profile.Steps) == 0 {
		v.appendColored("No profile data", colorStep)
		return
	}

	cursor := v.textEdit.TextCursor()

	for i, step := range v.profile.Steps {
		if i > 0 {
			cursor.InsertText("\n")
		}
		v.renderStep(cursor, i+1, step)
	}

	// Move cursor to top
	v.textEdit.MoveCursor(qt6.QTextCursor__Start)
}

// appendColored appends colored text (simple helper for single-line messages)
func (v *ProfileView) appendColored(text string, color *qt6.QColor) {
	cursor := v.textEdit.TextCursor()
	fmt := newCharFormat(color)
	cursor.InsertText2(text, fmt)
}

// newCharFormat creates a QTextCharFormat with the given foreground color
func newCharFormat(color *qt6.QColor) *qt6.QTextCharFormat {
	f := qt6.NewQTextCharFormat()
	f.SetForeground(qt6.NewQBrush3(color))
	return f
}

// renderStep renders a single profile step
func (v *ProfileView) renderStep(cursor *qt6.QTextCursor, stepNum int, step pack.Step) {
	switch s := step.(type) {
	case *pack.MethodStep:
		v.renderMethodStep(cursor, stepNum, s)
	case *pack.Method2Step:
		v.renderMethod2Step(cursor, stepNum, s)
	case *pack.SqlStep:
		v.renderSqlStepWithXType(cursor, stepNum, s, 0, 0)
	case *pack.SqlStep2:
		v.renderSqlStepWithXType(cursor, stepNum, &s.SqlStep, s.XType, 0)
	case *pack.SqlStep3:
		v.renderSqlStepWithXType(cursor, stepNum, &s.SqlStep, s.XType, s.Updated)
	case *pack.ApiCallStep:
		v.renderApiCallStep(cursor, stepNum, s)
	case *pack.ApiCallStep2:
		v.renderApiCallStep(cursor, stepNum, &s.ApiCallStep)
	case *pack.SocketStep:
		v.renderSocketStep(cursor, stepNum, s)
	case *pack.MessageStep:
		v.renderMessageStep(cursor, stepNum, s)
	case *pack.HashedMessageStep:
		v.renderHashedMessageStep(cursor, stepNum, s)
	case *pack.MethodSum:
		v.renderMethodSum(cursor, stepNum, s)
	case *pack.SqlSum:
		v.renderSqlSum(cursor, stepNum, s)
	case *pack.ApiCallSum:
		v.renderApiCallSum(cursor, stepNum, s)
	case *pack.SocketSum:
		v.renderSocketSum(cursor, stepNum, s)
	case *pack.ThreadSubmitStep:
		v.renderThreadSubmitStep(cursor, stepNum, s)
	case *pack.DispatchStep:
		v.renderDispatchStep(cursor, stepNum, s)
	case *pack.DumpStep:
		v.renderDumpStep(cursor, stepNum, s)
	case *pack.ParameterizedMessageStep:
		v.renderParameterizedMessageStep(cursor, stepNum, s)
	case *pack.ThreadCallPossibleStep:
		v.renderThreadCallPossibleStep(cursor, stepNum, s)
	case *pack.StepControl:
		v.renderStepControl(cursor, stepNum, s)
	default:
		v.writeStepHeader(cursor, stepNum, "Unknown", 0)
		cursor.InsertText2(fmt.Sprintf("type=%d", step.StepType()), newCharFormat(colorDefault))
	}
}

// writeStepHeader writes the common "[stepnum] elapsed type  " prefix
func (v *ProfileView) writeStepHeader(cursor *qt6.QTextCursor, stepNum int, typeName string, elapsedMs int32) {
	stepFmt := newCharFormat(colorStep)
	cursor.InsertText2(fmt.Sprintf("[%06d] ", stepNum), stepFmt)

	elapsedFmt := newCharFormat(colorElapsed)
	cursor.InsertText2(fmt.Sprintf("%6s ", formatElapsed(elapsedMs)), elapsedFmt)
}

// writeStepHeaderLong writes header with int64 elapsed (for summary steps)
func (v *ProfileView) writeStepHeaderLong(cursor *qt6.QTextCursor, stepNum int, typeName string, elapsedMs int64) {
	stepFmt := newCharFormat(colorStep)
	cursor.InsertText2(fmt.Sprintf("[%06d] ", stepNum), stepFmt)

	elapsedFmt := newCharFormat(colorElapsed)
	cursor.InsertText2(fmt.Sprintf("%6s ", formatElapsedLong(elapsedMs)), elapsedFmt)
}

// writeIndentedLine writes a new indented line (for child info like errors, params)
func (v *ProfileView) writeIndentedLine(cursor *qt6.QTextCursor) {
	cursor.InsertText("\n")
	cursor.InsertText2("                 ", newCharFormat(colorStep)) // 17 chars indent to align with details
}

func (v *ProfileView) renderMethodStep(cursor *qt6.QTextCursor, stepNum int, step *pack.MethodStep) {
	methodName := v.textCache.GetMethod(step.Hash)
	if methodName == "" {
		methodName = fmt.Sprintf("method#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "Method", step.Elapsed)
	cursor.InsertText2(methodName, newCharFormat(colorMethod))
}

func (v *ProfileView) renderMethod2Step(cursor *qt6.QTextCursor, stepNum int, step *pack.Method2Step) {
	v.renderMethodStep(cursor, stepNum, &step.MethodStep)

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderSqlStepWithXType(cursor *qt6.QTextCursor, stepNum int, step *pack.SqlStep, xtype byte, updated int32) {
	sqlText := v.textCache.GetSQL(step.Hash)
	if sqlText == "" {
		sqlText = fmt.Sprintf("sql#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "SQL", step.Elapsed)

	// xtype prefix
	prefix := sqlXTypePrefix(xtype)
	cursor.InsertText2(prefix, newCharFormat(colorStep))
	cursor.InsertText2(formatSQL(sqlText), newCharFormat(colorSQL))

	// Param as indented line
	if step.Param != "" {
		v.writeIndentedLine(cursor)
		cursor.InsertText2("PARAM "+step.Param, newCharFormat(colorParam))
	}

	// Updated/fetch info for SqlStep3
	if updated != 0 {
		v.writeIndentedLine(cursor)
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
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

// sqlXTypePrefix returns the SQL execution type prefix
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

func (v *ProfileView) renderApiCallStep(cursor *qt6.QTextCursor, stepNum int, step *pack.ApiCallStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("api#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "API", step.Elapsed)
	cursor.InsertText2(apiURL, newCharFormat(colorAPI))

	if step.Address != "" {
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ADDR "+step.Address, newCharFormat(colorParam))
	}

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderSocketStep(cursor *qt6.QTextCursor, stepNum int, step *pack.SocketStep) {
	ipAddr := formatIPAddr(step.IPAddr)
	v.writeStepHeader(cursor, stepNum, "Socket", step.Elapsed)
	cursor.InsertText2(fmt.Sprintf("%s:%d", ipAddr, step.Port), newCharFormat(colorSocket))

	if step.Error != 0 {
		errorText := v.textCache.GetError(step.Error)
		if errorText == "" {
			errorText = fmt.Sprintf("error#%d", step.Error)
		}
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ERROR "+errorText, newCharFormat(colorError))
	}
}

func (v *ProfileView) renderMessageStep(cursor *qt6.QTextCursor, stepNum int, step *pack.MessageStep) {
	v.writeStepHeader(cursor, stepNum, "Msg", 0)
	cursor.InsertText2(step.Message, newCharFormat(colorMessage))
}

func (v *ProfileView) renderHashedMessageStep(cursor *qt6.QTextCursor, stepNum int, step *pack.HashedMessageStep) {
	message := v.textCache.GetMessage(step.Hash)
	if message == "" {
		message = fmt.Sprintf("msg#%d", step.Hash)
	}

	displayText := message
	if step.Value != 0 {
		displayText = fmt.Sprintf("%s #%d", message, step.Value)
	}

	v.writeStepHeader(cursor, stepNum, "Msg", step.Time)
	cursor.InsertText2(displayText, newCharFormat(colorMessage))
}

func (v *ProfileView) renderMethodSum(cursor *qt6.QTextCursor, stepNum int, step *pack.MethodSum) {
	methodName := v.textCache.GetMethod(step.Hash)
	if methodName == "" {
		methodName = fmt.Sprintf("method#%d", step.Hash)
	}

	v.writeStepHeaderLong(cursor, stepNum, "MethodSum", step.Elapsed)
	cursor.InsertText2(fmt.Sprintf("%s (x%d)", methodName, step.Count), newCharFormat(colorSum))
}

func (v *ProfileView) renderSqlSum(cursor *qt6.QTextCursor, stepNum int, step *pack.SqlSum) {
	sqlText := v.textCache.GetSQL(step.Hash)
	if sqlText == "" {
		sqlText = fmt.Sprintf("sql#%d", step.Hash)
	}

	v.writeStepHeaderLong(cursor, stepNum, "SqlSum", step.Elapsed)
	cursor.InsertText2(fmt.Sprintf("%s (x%d)", formatSQL(sqlText), step.Count), newCharFormat(colorSum))
}

func (v *ProfileView) renderApiCallSum(cursor *qt6.QTextCursor, stepNum int, step *pack.ApiCallSum) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("api#%d", step.Hash)
	}

	v.writeStepHeaderLong(cursor, stepNum, "ApiSum", step.Elapsed)
	cursor.InsertText2(fmt.Sprintf("%s (x%d)", apiURL, step.Count), newCharFormat(colorSum))
}

func (v *ProfileView) renderSocketSum(cursor *qt6.QTextCursor, stepNum int, step *pack.SocketSum) {
	ipAddr := formatIPAddr(step.IPAddr)

	v.writeStepHeaderLong(cursor, stepNum, "SocketSum", step.Elapsed)
	cursor.InsertText2(fmt.Sprintf("%s:%d (x%d)", ipAddr, step.Port, step.Count), newCharFormat(colorSum))
}

func (v *ProfileView) renderThreadSubmitStep(cursor *qt6.QTextCursor, stepNum int, step *pack.ThreadSubmitStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("thread#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "Thread", step.Elapsed)
	cursor.InsertText2(apiURL, newCharFormat(colorThread))
}

func (v *ProfileView) renderDispatchStep(cursor *qt6.QTextCursor, stepNum int, step *pack.DispatchStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("dispatch#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "Dispatch", step.Elapsed)
	cursor.InsertText2(apiURL, newCharFormat(colorThread))

	if step.Address != "" {
		v.writeIndentedLine(cursor)
		cursor.InsertText2("ADDR "+step.Address, newCharFormat(colorParam))
	}
}

func (v *ProfileView) renderDumpStep(cursor *qt6.QTextCursor, stepNum int, step *pack.DumpStep) {
	v.writeStepHeader(cursor, stepNum, "Dump", 0)
	cursor.InsertText2(fmt.Sprintf("Thread: %s [%s]", step.ThreadName, step.ThreadState), newCharFormat(colorThread))
}

func (v *ProfileView) renderParameterizedMessageStep(cursor *qt6.QTextCursor, stepNum int, step *pack.ParameterizedMessageStep) {
	message := v.textCache.GetMessage(step.Hash)
	if message == "" {
		message = fmt.Sprintf("msg#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "ParamMsg", step.Elapsed)
	cursor.InsertText2(message, newCharFormat(colorMessage))

	if step.ParamString != "" {
		v.writeIndentedLine(cursor)
		cursor.InsertText2("PARAM "+step.ParamString, newCharFormat(colorParam))
	}
}

func (v *ProfileView) renderThreadCallPossibleStep(cursor *qt6.QTextCursor, stepNum int, step *pack.ThreadCallPossibleStep) {
	apiURL := v.textCache.GetAPICall(step.Hash)
	if apiURL == "" {
		apiURL = fmt.Sprintf("thread#%d", step.Hash)
	}

	v.writeStepHeader(cursor, stepNum, "ThreadCall", step.Elapsed)
	cursor.InsertText2(apiURL, newCharFormat(colorThread))
}

func (v *ProfileView) renderStepControl(cursor *qt6.QTextCursor, stepNum int, step *pack.StepControl) {
	v.writeStepHeader(cursor, stepNum, "Control", 0)
	cursor.InsertText2(step.Message, newCharFormat(colorControl))
}

// formatSQL formats SQL text with basic whitespace cleanup (no truncation for scrollable view)
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

// formatElapsedLong formats elapsed time from int64 milliseconds
func formatElapsedLong(ms int64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", float64(ms)/1000.0)
	}
	return fmt.Sprintf("%dms", ms)
}
