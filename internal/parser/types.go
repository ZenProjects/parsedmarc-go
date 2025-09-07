package parser

import (
	"encoding/json"
	"time"
)

// Storage interface for storing parsed reports
type Storage interface {
	StoreAggregateReport(report *AggregateReport) error
	StoreForensicReport(report *ForensicReport) error
	Close() error
}

// AggregateReport represents a parsed DMARC aggregate report
type AggregateReport struct {
	XMLSchema      string                `json:"xml_schema"`
	ReportMetadata ReportMetadata        `json:"report_metadata"`
	PolicyPublished PolicyPublished      `json:"policy_published"`
	Records        []Record             `json:"records"`
}

// ReportMetadata contains metadata about the report
type ReportMetadata struct {
	OrgName                string    `json:"org_name"`
	OrgEmail               string    `json:"org_email"`
	OrgExtraContactInfo    *string   `json:"org_extra_contact_info"`
	ReportID               string    `json:"report_id"`
	BeginDate              time.Time `json:"begin_date"`
	EndDate                time.Time `json:"end_date"`
	Errors                 []string  `json:"errors"`
}

// PolicyPublished represents the DMARC policy that was published
type PolicyPublished struct {
	Domain string `json:"domain"`
	ADKIM  string `json:"adkim"`
	ASPF   string `json:"aspf"`
	P      string `json:"p"`
	SP     string `json:"sp"`
	PCT    string `json:"pct"`
	FO     string `json:"fo"`
}

// Record represents a single record from the aggregate report
type Record struct {
	Source          Source           `json:"source"`
	Count           int              `json:"count"`
	Alignment       Alignment        `json:"alignment"`
	PolicyEvaluated PolicyEvaluated  `json:"policy_evaluated"`
	Identifiers     Identifiers      `json:"identifiers"`
	AuthResults     AuthResults      `json:"auth_results"`
}

// Source contains information about the source IP
type Source struct {
	IPAddress    string  `json:"ip_address"`
	Country      string  `json:"country"`
	ReverseDNS   string  `json:"reverse_dns"`
	BaseDomain   string  `json:"base_domain"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
}

// Alignment indicates SPF, DKIM and overall DMARC alignment
type Alignment struct {
	SPF   bool `json:"spf"`
	DKIM  bool `json:"dkim"`
	DMARC bool `json:"dmarc"`
}

// PolicyEvaluated shows the results of policy evaluation
type PolicyEvaluated struct {
	Disposition              string                   `json:"disposition"`
	DKIM                    string                   `json:"dkim"`
	SPF                     string                   `json:"spf"`
	PolicyOverrideReasons   []PolicyOverrideReason   `json:"policy_override_reasons"`
}

// PolicyOverrideReason describes why policy was overridden
type PolicyOverrideReason struct {
	Type    *string `json:"type"`
	Comment *string `json:"comment"`
}

// Identifiers contains header and envelope information
type Identifiers struct {
	HeaderFrom   string  `json:"header_from"`
	EnvelopeFrom *string `json:"envelope_from"`
	EnvelopeTo   *string `json:"envelope_to"`
}

// AuthResults contains SPF and DKIM authentication results
type AuthResults struct {
	DKIM []DKIMResult `json:"dkim"`
	SPF  []SPFResult  `json:"spf"`
}

// DKIMResult represents a DKIM authentication result
type DKIMResult struct {
	Domain   string `json:"domain"`
	Selector string `json:"selector"`
	Result   string `json:"result"`
}

// SPFResult represents an SPF authentication result
type SPFResult struct {
	Domain string `json:"domain"`
	Scope  string `json:"scope"`
	Result string `json:"result"`
}

// ForensicReport represents a parsed DMARC forensic report
type ForensicReport struct {
	FeedbackType                string              `json:"feedback_type"`
	UserAgent                   *string             `json:"user_agent"`
	Version                     *string             `json:"version"`
	OriginalEnvelopeID         *string             `json:"original_envelope_id"`
	OriginalMailFrom           *string             `json:"original_mail_from"`
	OriginalRcptTo             *string             `json:"original_rcpt_to"`
	ArrivalDate                time.Time           `json:"arrival_date"`
	ArrivalDateUTC             time.Time           `json:"arrival_date_utc"`
	Subject                    string              `json:"subject"`
	MessageID                  string              `json:"message_id"`
	AuthenticationResults      string              `json:"authentication_results"`
	DKIMDomain                 *string             `json:"dkim_domain"`
	Source                     Source              `json:"source"`
	DeliveryResult             string              `json:"delivery_result"`
	AuthFailure                []string            `json:"auth_failure"`
	ReportedDomain             string              `json:"reported_domain"`
	AuthenticationMechanisms   []string            `json:"authentication_mechanisms"`
	SampleHeadersOnly          bool                `json:"sample_headers_only"`
	Sample                     string              `json:"sample"`
	ParsedSample              json.RawMessage      `json:"parsed_sample"`
}

// SMTPTLSReport represents a parsed SMTP TLS report
type SMTPTLSReport struct {
	OrganizationName string             `json:"organization_name"`
	BeginDate        time.Time          `json:"begin_date"`
	EndDate          time.Time          `json:"end_date"`
	ContactInfo      string             `json:"contact_info"`
	ReportID         string             `json:"report_id"`
	Policies         []SMTPTLSPolicy    `json:"policies"`
}

// SMTPTLSPolicy represents a policy in SMTP TLS report
type SMTPTLSPolicy struct {
	PolicyDomain              string                    `json:"policy_domain"`
	PolicyType               string                    `json:"policy_type"`
	PolicyStrings            []string                  `json:"policy_strings,omitempty"`
	MXHostPatterns           []string                  `json:"mx_host_patterns,omitempty"`
	SuccessfulSessionCount   int                       `json:"successful_session_count"`
	FailedSessionCount       int                       `json:"failed_session_count"`
	FailureDetails           []SMTPTLSFailureDetails   `json:"failure_details,omitempty"`
}

// SMTPTLSFailureDetails contains details about TLS failures
type SMTPTLSFailureDetails struct {
	ResultType               string  `json:"result_type"`
	FailedSessionCount       int     `json:"failed_session_count"`
	SendingMTAIP            *string `json:"sending_mta_ip,omitempty"`
	ReceivingIP             *string `json:"receiving_ip,omitempty"`
	ReceivingMXHostname     *string `json:"receiving_mx_hostname,omitempty"`
	ReceivingMXHelo         *string `json:"receiving_mx_helo,omitempty"`
	AdditionalInfoURI       *string `json:"additional_info_uri,omitempty"`
	FailureReasonCode       *string `json:"failure_reason_code,omitempty"`
}