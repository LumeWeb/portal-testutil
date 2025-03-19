package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal/core"
)

func TestMailerTestHelper(t *testing.T) {
	// Skip this test until we have proper mocks for the dependencies
	t.Skip("Skipping mailer test helper test until proper mocks are available")

	// Create test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	// Create mailer test helper
	mailerHelper := NewMailerTestHelper(t)

	// Register test templates
	mailerHelper.RegisterTemplate(
		"test_template",
		"Test Subject: {{.Subject}}",
		"Test Body: {{.Body}}")

	// Register the mailer with the test context
	mailerHelper.RegisterWithContext(tc)

	// Set up expectations
	mailerHelper.ExpectTemplateSend("test_template", "test@example.com").Return(nil)

	// Get the mailer directly from the helper
	mailer := mailerHelper.GetMailer()
	assert.NotNil(t, mailer)

	// Send an email
	subjectVars := core.MailerTemplateData{"Subject": "Hello"}
	bodyVars := core.MailerTemplateData{"Body": "World"}
	err := mailer.TemplateSend("test_template", subjectVars, bodyVars, "test@example.com")
	assert.NoError(t, err)

	// Assert that the email was sent
	email := mailerHelper.AssertEmailSent("test_template", "test@example.com")
	assert.NotNil(t, email)

	// Assert the email content
	mailerHelper.AssertEmailContent(email, "Test Subject: Hello", "Test Body: World")

	// Verify that the variables were passed correctly
	assert.Equal(t, "Hello", email.SubjectVars["Subject"])
	assert.Equal(t, "World", email.BodyVars["Body"])

	// Test that no email was sent to a different address
	mailerHelper.AssertNoEmailSent("test_template", "other@example.com")
}
