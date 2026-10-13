package tui

import (
	"context"
	"time"

	"github.com/UruhaLushia/headdeck/internal/i18n"
	"github.com/UruhaLushia/headdeck/internal/ipc/client"
	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type page int

const (
	pageStatus page = iota
	pageNodes
	pageNodeDetail
	pageUsers
	pageSettings
	pageDNS
	pageHeadscale
	pagePolicy
	pageKeys
	pageAccounts
	pageProviders
)

var pageNames = map[page]string{
	pageStatus:     "Overview",
	pageNodes:      "Nodes",
	pageNodeDetail: "Node Details",
	pageUsers:      "Users",
	pageSettings:   "Settings",
	pageDNS:        "DNS",
	pageHeadscale:  "Configuration",
	pagePolicy:     "ACL",
	pageKeys:       "Auth keys",
	pageAccounts:   "Access accounts",
	pageProviders:  "Login providers",
}

type connState int

const (
	connUnknown connState = iota
	connConnecting
	connReady
	connError
)

type Model struct {
	client         *client.Client
	requestCtx     context.Context
	cancelRequests context.CancelFunc
	socket         string
	authorization  string

	width        int
	height       int
	page         page
	conn         connState
	err          error
	quitting     bool
	locale       i18n.Locale
	focus        focusArea
	actionCursor int

	settings           *protocol.Settings
	settingsGroup      string
	managedGroup       string
	dns                *protocol.DNSSettings
	managed            *protocol.ManagedConfig
	managedGen         int
	managedLoading     bool
	policy             *protocol.PolicySettings
	policyGen          int
	policyLoading      bool
	objectForm         *objectForm
	dnsGen             int
	dnsLoading         bool
	settingPopup       *settingSelection
	confirmation       *resourceConfirmation
	menu               []resourceAction
	menuCursor         int
	resource           *resourceAction
	resourceBusy       bool
	serviceOperation   *protocol.Operation
	notice             string
	settingsGen        int
	editKey, editValue string
	editCursor         int
	mouseX, mouseY     int
	scrollDragging     bool
	lastClick          time.Time
	lastClickRow       int
	lastClickPage      page
	hello              *protocol.HelloResponse
	status             *protocol.Status
	statusReceivedAt   time.Time
	nodes              []protocol.Node
	users              []protocol.User
	keys               []protocol.PreAuthKey
	keysGen            int
	keysLoading        bool
	keysLoaded         bool
	accounts           []protocol.Account
	accountsGen        int
	accountsLoading    bool
	accountSecurity    *protocol.AccountSecurity
	providers          []protocol.LoginProvider
	providersGen       int
	providersLoading   bool
	detail             *protocol.Node
	cursor             int
	listOffset         int
	detailOffset       int
	filter             string
	searching          bool

	// Reject responses from obsolete refresh generations.
	nodesGen             int
	usersGen             int
	statGen              int
	statusLoading        bool
	statusRefreshPending bool
	nodesLoading         bool
	usersLoading         bool
	nodesRevision        uint64
	usersRevision        uint64
	nodesWantedRevision  uint64
	usersWantedRevision  uint64
	pollTick             uint64

	streamCh         chan streamMsg
	streamCancel     context.CancelFunc
	streamContext    context.Context
	streamDone       chan struct{}
	streamGeneration uint64
	streamWaiting    bool
	streamConnected  bool
	lastEventID      uint64
	pendingEvent     bool
	debounceActive   bool

	now func() time.Time
}

func NewModel(c *client.Client, opts Options) *Model {
	locale, _ := i18n.Resolve(opts.Language)
	return &Model{
		locale:        locale,
		client:        c,
		socket:        opts.Socket,
		authorization: opts.Authorization,
		page:          pageStatus,
		focus:         focusNavigation,
		conn:          connConnecting,
		streamCh:      make(chan streamMsg, 32),
		now:           time.Now,
		mouseX:        -1,
		mouseY:        -1,
	}
}
