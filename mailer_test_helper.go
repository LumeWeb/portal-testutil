package testutil

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"text/template"

	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/service"
)

// TestMailerService implements core.MailerService for testing
type TestMailerService struct {
	mockObj    mock.Mock
	templates  map[string]core.MailerTemplate
	sentEmails []TestEmail
	mu         sync.Mutex
}

// TestEmail represents a captured email
type TestEmail struct {
	Template        string
	SubjectVars     core.MailerTemplateData
	BodyVars        core.MailerTemplateData
	To              string
	RenderedSubject string
	RenderedBody    string
}

// NewTestMailerService creates a new test mailer service
func NewTestMailerService() *TestMailerService {
	return &TestMailerService{
		templates:  make(map[string]core.MailerTemplate),
		sentEmails: make([]TestEmail, 0),
	}
}

// ID implements the Service interface
func (m *TestMailerService) ID() string {
	return core.MAILER_SERVICE
}

// Config implements the Service interface
func (m *TestMailerService) Config() (any, error) {
	args := m.mockObj.Called()
	return args.Get(0), args.Error(1)
}

// TemplateSend implements the MailerService interface
func (m *TestMailerService) TemplateSend(template string, subjectVars core.MailerTemplateData, bodyVars core.MailerTemplateData, to string) error {
	args := m.mockObj.Called(template, subjectVars, bodyVars, to)

	// Render the template if it exists
	var renderedSubject, renderedBody string
	if tmpl, exists := m.templates[template]; exists {
		var subjectBuf, bodyBuf bytes.Buffer

		if err := tmpl.Subject().Execute(&subjectBuf, subjectVars); err == nil {
			renderedSubject = subjectBuf.String()
		}

		if err := tmpl.Body().Execute(&bodyBuf, bodyVars); err == nil {
			renderedBody = bodyBuf.String()
		}
	}

	// Capture the email
	m.mu.Lock()
	m.sentEmails = append(m.sentEmails, TestEmail{
		Template:        template,
		SubjectVars:     subjectVars,
		BodyVars:        bodyVars,
		To:              to,
		RenderedSubject: renderedSubject,
		RenderedBody:    renderedBody,
	})
	m.mu.Unlock()

	return args.Error(0)
}

// TemplateRegister implements the MailerService interface
func (m *TestMailerService) TemplateRegister(name string, template core.MailerTemplate) error {
	args := m.mockObj.Called(name, template)
	m.templates[name] = template
	return args.Error(0)
}

// Mock implements the ServiceMockConfigurable interface
func (m *TestMailerService) Mock() *mock.Mock {
	return &m.mockObj
}

// GetSentEmails returns all captured emails
func (m *TestMailerService) GetSentEmails() []TestEmail {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Return a copy to avoid race conditions
	emails := make([]TestEmail, len(m.sentEmails))
	copy(emails, m.sentEmails)
	return emails
}

// ClearSentEmails clears the captured emails
func (m *TestMailerService) ClearSentEmails() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentEmails = make([]TestEmail, 0)
}

// GetTemplate returns a registered template
func (m *TestMailerService) GetTemplate(name string) (core.MailerTemplate, bool) {
	template, exists := m.templates[name]
	return template, exists
}

// MailerTestHelper provides utilities for testing email functionality
type MailerTestHelper struct {
	mailer *TestMailerService
	t      *testing.T
}

// NewMailerTestHelper creates a new mailer test helper
func NewMailerTestHelper(t *testing.T) *MailerTestHelper {
	return &MailerTestHelper{
		mailer: NewTestMailerService(),
		t:      t,
	}
}

// GetMailer returns the test mailer service
func (h *MailerTestHelper) GetMailer() *TestMailerService {
	return h.mailer
}

// RegisterWithContext registers the test mailer with the test context
func (h *MailerTestHelper) RegisterWithContext(tc *DBTestContext) {
	tc.RegisterService(core.MAILER_SERVICE, h.mailer)
}

// ExpectTemplateSend sets up an expectation for a template send
func (h *MailerTestHelper) ExpectTemplateSend(template string, to string) *mock.Call {
	return h.mailer.mockObj.On("TemplateSend", template, mock.Anything, mock.Anything, to)
}

// ExpectTemplateRegister sets up an expectation for a template registration
func (h *MailerTestHelper) ExpectTemplateRegister(name string) *mock.Call {
	return h.mailer.mockObj.On("TemplateRegister", name, mock.Anything)
}

// AssertEmailSent asserts that an email was sent with the given template and to address
func (h *MailerTestHelper) AssertEmailSent(template string, to string) *TestEmail {
	emails := h.mailer.GetSentEmails()
	for _, email := range emails {
		if email.Template == template && email.To == to {
			return &email
		}
	}

	h.t.Errorf("No email sent with template '%s' to '%s'", template, to)
	return nil
}

// AssertEmailContent asserts that an email contains the expected content
func (h *MailerTestHelper) AssertEmailContent(email *TestEmail, subjectContains, bodyContains string) {
	if email == nil {
		h.t.Error("Cannot assert content on nil email")
		return
	}

	if subjectContains != "" && !strings.Contains(email.RenderedSubject, subjectContains) {
		h.t.Errorf("Email subject does not contain '%s', got: '%s'",
			subjectContains, email.RenderedSubject)
	}

	if bodyContains != "" && !strings.Contains(email.RenderedBody, bodyContains) {
		h.t.Errorf("Email body does not contain '%s', got: '%s'",
			bodyContains, email.RenderedBody)
	}
}

// AssertNoEmailSent asserts that no email was sent with the given template and to address
func (h *MailerTestHelper) AssertNoEmailSent(template string, to string) {
	emails := h.mailer.GetSentEmails()
	for _, email := range emails {
		if email.Template == template && email.To == to {
			h.t.Errorf("Email was sent with template '%s' to '%s', but none was expected",
				template, to)
			return
		}
	}
}

// RegisterTemplate registers a template with the test mailer
func (h *MailerTestHelper) RegisterTemplate(name, subjectTemplate, bodyTemplate string) {
	subject := template.Must(template.New(name + "_subject").Parse(subjectTemplate))
	body := template.Must(template.New(name + "_body").Parse(bodyTemplate))

	tmpl := service.NewMailerTemplate(subject, body)
	err := h.mailer.TemplateRegister(name, tmpl)
	if err != nil {
		h.t.Fatalf("Failed to register template '%s': %v", name, err)
	}
}

// init registers the TestMailerService with the global registry
func init() {
	RegisterMockFactory(core.MAILER_SERVICE, func() core.Service {
		return NewTestMailerService()
	})
}
