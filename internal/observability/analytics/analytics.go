package analytics

// Product telemetry is removed; this compatibility surface never leaves the app.
type Client struct{}

const DefaultHost = ""

type Request struct {
	// UserID is the account the event belongs to, and becomes the distinct id.
	// Empty means nobody is named and the event lands on the cookieless hash.
	UserID string
	// Email and Name are set on the person when UserID is given.
	Email string
	Name  string
	// OrganizationID puts the event in the workspace's group; OrganizationName
	// and Plan are set on that group.
	OrganizationID   string
	OrganizationName string
	Plan             string
	// SetOnce are person properties written the first time only, which is
	// what acquisition is: where somebody came from does not change later.
	SetOnce map[string]any

	// IP and UserAgent are the originating browser request, forwarded so the
	// event is geolocated and, without a UserID, so the cookieless hash lands
	// on the same visitor as that browser's own events.
	IP        string
	UserAgent string
	// Host is the site the visitor was on, one of the cookieless hash inputs.
	// local logging reduces it to the registrable root domain, so example.test
	// and example.test hash alike.
	Host string
}

func New(_, _ string) *Client                                   { return nil }
func (c *Client) Capture(_ string, _ Request, _ map[string]any) {}
