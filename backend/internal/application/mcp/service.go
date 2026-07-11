package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
	domainsystem "github.com/hjyoon/ogame-opensource/backend/internal/domain/system"
)

var ErrInvalidTokenRequest = errors.New("invalid mcp token request")
var ErrInvalidOAuthRequest = errors.New("invalid mcp oauth request")
var ErrInvalidOAuthGrant = errors.New("invalid mcp oauth grant")

const (
	defaultMCPTokenTTL = 30 * 24 * time.Hour
	mcpOAuthCodeTTL    = 10 * time.Minute
)

type HealthProvider interface {
	Get(context.Context) domainsystem.Health
}

type TokenVerifier interface {
	VerifyMCPToken(context.Context, string) (domainmcp.Access, error)
}

type ToolCallAuditor interface {
	RecordMCPToolCall(context.Context, domainmcp.ToolCallAudit)
}

type OIDCSigner interface {
	Algorithm() string
	JWKS(context.Context) domainmcp.JSONWebKeySet
	SignIDToken(context.Context, domainmcp.IDTokenCommand) (string, error)
}

type SessionLookup interface {
	GetGameSession(context.Context, apppublicsite.GameSessionCommand) (domainpublicsite.SessionAuthentication, error)
}

type TokenRepository interface {
	ListMCPTokens(context.Context, int) ([]domainmcp.Token, error)
	CreateMCPToken(context.Context, domainmcp.Token, string) (domainmcp.Token, error)
	RevokeMCPToken(context.Context, int, int, int64) (bool, error)
	RevokeMCPTokenByHash(context.Context, string, int64) (bool, error)
}

type OAuthCodeRepository interface {
	CreateMCPOAuthCode(context.Context, domainmcp.OAuthAuthorizationCode) (domainmcp.OAuthAuthorizationCode, error)
	ConsumeMCPOAuthCode(context.Context, string, int64) (domainmcp.OAuthAuthorizationCode, error)
}

type ReadRepository interface {
	ListMCPPlanets(context.Context, int) ([]domainmcp.Planet, error)
	GetMCPAccountOverview(context.Context, int) (domainmcp.AccountOverview, error)
	GetMCPPlanetResources(context.Context, int, int) (domainmcp.PlanetResources, error)
	GetMCPBuildingQueue(context.Context, int, int) (domainmcp.BuildingQueue, error)
	ListMCPMessages(context.Context, int, domainmcp.MessageQuery) (domainmcp.MessageList, error)
	GetMCPMessage(context.Context, int, int) (domainmcp.MessageDetail, error)
	GetMCPFleetMovements(context.Context, int) (domainmcp.FleetMovements, error)
}

type ReportReadRepository interface {
	GetMCPReport(context.Context, int, domainmcp.ReportCommand) (domainmcp.Report, error)
}

type WriteRepository interface {
	PreviewMCPSendMessage(context.Context, int, domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error)
	SendMCPMessage(context.Context, int, domainmcp.SendMessageCommand) (domainmcp.SendMessageResult, error)
	PreviewMCPDeleteMessages(context.Context, int, domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error)
	DeleteMCPMessages(context.Context, int, domainmcp.DeleteMessagesCommand) (domainmcp.DeleteMessagesResult, error)
	PreviewMCPReportMessage(context.Context, int, domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error)
	ReportMCPMessage(context.Context, int, domainmcp.ReportMessageCommand) (domainmcp.ReportMessageResult, error)
}

type FleetWriteRepository interface {
	PreviewMCPDispatchFleet(context.Context, int, domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error)
	DispatchMCPFleet(context.Context, int, domainmcp.DispatchFleetCommand) (domainmcp.DispatchFleetValidationResult, error)
	PreviewMCPRecallFleet(context.Context, int, domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error)
	RecallMCPFleet(context.Context, int, domainmcp.RecallFleetCommand) (domainmcp.RecallFleetResult, error)
}

type PhalanxWriteRepository interface {
	PreviewMCPPhalanxScan(context.Context, int, domainmcp.PhalanxScanCommand) (domainmcp.PhalanxScanResult, error)
	ScanMCPPhalanx(context.Context, int, domainmcp.PhalanxScanCommand) (domainmcp.PhalanxScanResult, error)
}

type QueueWriteRepository interface {
	PreviewMCPCancelBuildingQueue(context.Context, int, domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error)
	CancelMCPBuildingQueue(context.Context, int, domainmcp.CancelBuildingQueueCommand) (domainmcp.CancelBuildingQueueResult, error)
	PreviewMCPCancelResearchQueue(context.Context, int, domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error)
	CancelMCPResearchQueue(context.Context, int, domainmcp.CancelResearchQueueCommand) (domainmcp.CancelResearchQueueResult, error)
	PreviewMCPEnqueueShipyardOrder(context.Context, int, domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error)
	EnqueueMCPShipyardOrder(context.Context, int, domainmcp.EnqueueShipyardOrderCommand) (domainmcp.EnqueueShipyardOrderResult, error)
}

type ResourceWriteRepository interface {
	PreviewMCPUpdateResourceProduction(context.Context, int, domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error)
	UpdateMCPResourceProduction(context.Context, int, domainmcp.UpdateResourceProductionCommand) (domainmcp.UpdateResourceProductionResult, error)
}

type ResourceProductionReadRepository interface {
	GetMCPResourceProductionOptions(context.Context, int, int) (domainmcp.ResourceProductionOptions, error)
}

type PremiumReadRepository interface {
	GetMCPOfficerStatus(context.Context, int, int) (domainmcp.OfficerStatus, error)
}

type SearchReadRepository interface {
	SearchMCP(context.Context, int, domainmcp.SearchCommand) (domainmcp.SearchResult, error)
}

type GalaxyReadRepository interface {
	GetMCPGalaxySystem(context.Context, int, domainmcp.GalaxySystemCommand) (domainmcp.GalaxySystem, error)
}

type StatisticsReadRepository interface {
	GetMCPStatistics(context.Context, int, domainmcp.StatisticsCommand) (domainmcp.Statistics, error)
}

type AllianceReadRepository interface {
	GetMCPAllianceStatus(context.Context, int, domainmcp.AllianceStatusCommand) (domainmcp.AllianceStatus, error)
}

type BuddyReadRepository interface {
	GetMCPBuddyStatus(context.Context, int, domainmcp.BuddyStatusCommand) (domainmcp.BuddyStatus, error)
}

type BuddyWriteRepository interface {
	PreviewMCPBuddyMutation(context.Context, int, domainmcp.BuddyMutationCommand) (domainmcp.BuddyMutationResult, error)
	MutateMCPBuddy(context.Context, int, domainmcp.BuddyMutationCommand) (domainmcp.BuddyMutationResult, error)
}

type PrangerReadRepository interface {
	GetMCPPranger(context.Context, int, domainmcp.PrangerCommand) (domainmcp.Pranger, error)
}

type NotesReadRepository interface {
	GetMCPNotes(context.Context, int, domainmcp.NotesStatusCommand) (domainmcp.NotesStatus, error)
}

type NotesWriteRepository interface {
	PreviewMCPCreateNote(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
	CreateMCPNote(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
	PreviewMCPUpdateNote(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
	UpdateMCPNote(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
	PreviewMCPDeleteNotes(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
	DeleteMCPNotes(context.Context, int, domainmcp.NoteMutationCommand) (domainmcp.NoteMutationResult, error)
}

type OptionsReadRepository interface {
	GetMCPOptions(context.Context, int, domainmcp.OptionsStatusCommand) (domainmcp.OptionsStatus, error)
}

type MaintenanceReadRepository interface {
	GetMCPMaintenance(context.Context, int) (domainmcp.MaintenanceStatus, error)
}

type MerchantReadRepository interface {
	GetMCPMerchantStatus(context.Context, int, domainmcp.MerchantStatusCommand) (domainmcp.MerchantStatus, error)
}

type MerchantWriteRepository interface {
	PreviewMCPMutateMerchant(context.Context, int, domainmcp.MerchantMutationCommand) (domainmcp.MerchantMutationResult, error)
	MutateMCPMerchant(context.Context, int, domainmcp.MerchantMutationCommand) (domainmcp.MerchantMutationResult, error)
}

type JumpGateReadRepository interface {
	GetMCPJumpGateStatus(context.Context, int, domainmcp.JumpGateStatusCommand) (domainmcp.JumpGateStatus, error)
}

type JumpGateWriteRepository interface {
	PreviewMCPJumpGate(context.Context, int, domainmcp.JumpGateCommand) (domainmcp.JumpGateResult, error)
	JumpMCPJumpGate(context.Context, int, domainmcp.JumpGateCommand) (domainmcp.JumpGateResult, error)
}

type EmpireReadRepository interface {
	GetMCPEmpire(context.Context, int, domainmcp.EmpireCommand) (domainmcp.EmpireOverview, error)
}

type TechnologyReadRepository interface {
	GetMCPTechnology(context.Context, int, domainmcp.TechnologyCommand) (domainmcp.TechnologyTree, error)
}

type BuildingOptionsReadRepository interface {
	GetMCPBuildingOptions(context.Context, int, int) (domainmcp.BuildingOptions, error)
}

type ResearchOptionsReadRepository interface {
	GetMCPResearchOptions(context.Context, int, int) (domainmcp.ResearchOptions, error)
}

type ShipyardOptionsReadRepository interface {
	GetMCPShipyardOptions(context.Context, int, int) (domainmcp.ShipyardOptions, error)
}

type DefenseOptionsReadRepository interface {
	GetMCPDefenseOptions(context.Context, int, int) (domainmcp.DefenseOptions, error)
}

type FleetOptionsReadRepository interface {
	GetMCPFleetOptions(context.Context, int, int) (domainmcp.FleetOptions, error)
}

type PremiumWriteRepository interface {
	PreviewMCPRecruitOfficer(context.Context, int, domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error)
	RecruitMCPOfficer(context.Context, int, domainmcp.RecruitOfficerCommand) (domainmcp.RecruitOfficerResult, error)
}

type TokenSecretGenerator interface {
	NewMCPToken() (string, error)
}

type OAuthCodeGenerator interface {
	NewMCPOAuthCode() (string, error)
}

type SecureTokenGenerator struct{}

func (SecureTokenGenerator) NewMCPToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ogmcp_" + hex.EncodeToString(bytes), nil
}

func (SecureTokenGenerator) NewMCPOAuthCode() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ogmcp_code_" + hex.EncodeToString(bytes), nil
}

type TokenManagementCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
}

type CreateTokenCommand struct {
	TokenManagementCommand
	Name   string
	Scopes []string
}

type RevokeTokenCommand struct {
	TokenManagementCommand
	TokenID int
}

type OAuthAuthorizeCommand struct {
	TokenManagementCommand
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Resource            string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	ConsentApproved     bool
	Issuer              string
}

type OAuthAuthorizeResult struct {
	Authenticated   bool
	Issues          []domainpublicsite.SessionIssue
	RequiresConsent bool
	ClientID        string
	RedirectURI     string
	Scopes          []string
	RedirectTo      string
	Code            string
}

type OAuthClientRegistrationCommand struct {
	RedirectURIs            []string
	ClientName              string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
	Scope                   string
}

type OAuthClientRegistrationResult struct {
	ClientID                string
	ClientIDIssuedAt        int64
	ClientName              string
	RedirectURIs            []string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
	Scope                   string
}

type OAuthTokenCommand struct {
	GrantType    string
	Code         string
	RedirectURI  string
	Resource     string
	ClientID     string
	CodeVerifier string
	Issuer       string
}

type OAuthRevokeCommand struct {
	Token         string
	TokenTypeHint string
}

type OAuthTokenResult struct {
	AccessToken string
	TokenType   string
	Scope       string
	IDToken     string
	ExpiresIn   int64
	Token       domainmcp.Token
}

type OAuthRevokeResult struct {
	Revoked bool
}

type TokenListResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Tokens        []domainmcp.Token
}

type TokenCreationResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Creation      domainmcp.TokenCreation
}

type TokenRevokeResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Revoked       bool
}

type Service struct {
	health          HealthProvider
	verifier        TokenVerifier
	tokenRepository TokenRepository
	oauthRepository OAuthCodeRepository
	oauthRedirects  []string
	oidcSigner      OIDCSigner
	readRepository  ReadRepository
	reportRead      ReportReadRepository
	writeRepository WriteRepository
	fleetWrite      FleetWriteRepository
	phalanxWrite    PhalanxWriteRepository
	queueWrite      QueueWriteRepository
	resourceWrite   ResourceWriteRepository
	resourceRead    ResourceProductionReadRepository
	premiumRead     PremiumReadRepository
	searchRead      SearchReadRepository
	galaxyRead      GalaxyReadRepository
	statisticsRead  StatisticsReadRepository
	allianceRead    AllianceReadRepository
	buddyRead       BuddyReadRepository
	buddyWrite      BuddyWriteRepository
	prangerRead     PrangerReadRepository
	notesRead       NotesReadRepository
	notesWrite      NotesWriteRepository
	optionsRead     OptionsReadRepository
	maintenanceRead MaintenanceReadRepository
	merchantRead    MerchantReadRepository
	merchantWrite   MerchantWriteRepository
	jumpGateRead    JumpGateReadRepository
	jumpGateWrite   JumpGateWriteRepository
	empireRead      EmpireReadRepository
	technologyRead  TechnologyReadRepository
	buildingRead    BuildingOptionsReadRepository
	researchRead    ResearchOptionsReadRepository
	shipyardRead    ShipyardOptionsReadRepository
	defenseRead     DefenseOptionsReadRepository
	fleetOptions    FleetOptionsReadRepository
	premiumWrite    PremiumWriteRepository
	sessions        SessionLookup
	tokenGenerator  TokenSecretGenerator
	codeGenerator   OAuthCodeGenerator
	auditor         ToolCallAuditor
	tokenTTL        time.Duration
	now             func() time.Time
}

func NewService(health HealthProvider) Service {
	return Service{health: health, now: time.Now}
}

func NewServiceWithTokenVerifier(health HealthProvider, verifier TokenVerifier) Service {
	return Service{health: health, verifier: verifier, now: time.Now}
}

func NewServiceWithTokenManagement(health HealthProvider, verifier TokenVerifier, repository TokenRepository, sessions SessionLookup, generator TokenSecretGenerator, now func() time.Time) Service {
	if generator == nil {
		generator = SecureTokenGenerator{}
	}
	codeGenerator, ok := generator.(OAuthCodeGenerator)
	if !ok {
		codeGenerator = SecureTokenGenerator{}
	}
	if now == nil {
		now = time.Now
	}
	return Service{
		health:          health,
		verifier:        verifier,
		tokenRepository: repository,
		sessions:        sessions,
		tokenGenerator:  generator,
		codeGenerator:   codeGenerator,
		tokenTTL:        defaultMCPTokenTTL,
		now:             now,
	}
}

func (s Service) WithOAuthCodeRepository(repository OAuthCodeRepository) Service {
	s.oauthRepository = repository
	return s
}

func (s Service) WithOAuthRedirectURIs(redirectURIs []string) Service {
	s.oauthRedirects = normalizeAllowedOAuthRedirectURIs(redirectURIs)
	return s
}

func (s Service) WithOIDCSigner(signer OIDCSigner) Service {
	s.oidcSigner = signer
	return s
}

func (s Service) WithReadRepository(repository ReadRepository) Service {
	s.readRepository = repository
	return s
}

func (s Service) WithReportReadRepository(repository ReportReadRepository) Service {
	s.reportRead = repository
	return s
}

func (s Service) WithWriteRepository(repository WriteRepository) Service {
	s.writeRepository = repository
	return s
}

func (s Service) WithFleetWriteRepository(repository FleetWriteRepository) Service {
	s.fleetWrite = repository
	return s
}

func (s Service) WithPhalanxWriteRepository(repository PhalanxWriteRepository) Service {
	s.phalanxWrite = repository
	return s
}

func (s Service) WithQueueWriteRepository(repository QueueWriteRepository) Service {
	s.queueWrite = repository
	return s
}

func (s Service) WithResourceWriteRepository(repository ResourceWriteRepository) Service {
	s.resourceWrite = repository
	return s
}

func (s Service) WithResourceProductionReadRepository(repository ResourceProductionReadRepository) Service {
	s.resourceRead = repository
	return s
}

func (s Service) WithPremiumReadRepository(repository PremiumReadRepository) Service {
	s.premiumRead = repository
	return s
}

func (s Service) WithSearchReadRepository(repository SearchReadRepository) Service {
	s.searchRead = repository
	return s
}

func (s Service) WithGalaxyReadRepository(repository GalaxyReadRepository) Service {
	s.galaxyRead = repository
	return s
}

func (s Service) WithStatisticsReadRepository(repository StatisticsReadRepository) Service {
	s.statisticsRead = repository
	return s
}

func (s Service) WithAllianceReadRepository(repository AllianceReadRepository) Service {
	s.allianceRead = repository
	return s
}

func (s Service) WithBuddyReadRepository(repository BuddyReadRepository) Service {
	s.buddyRead = repository
	return s
}

func (s Service) WithBuddyWriteRepository(repository BuddyWriteRepository) Service {
	s.buddyWrite = repository
	return s
}

func (s Service) WithPrangerReadRepository(repository PrangerReadRepository) Service {
	s.prangerRead = repository
	return s
}

func (s Service) WithNotesReadRepository(repository NotesReadRepository) Service {
	s.notesRead = repository
	return s
}

func (s Service) WithNotesWriteRepository(repository NotesWriteRepository) Service {
	s.notesWrite = repository
	return s
}

func (s Service) WithOptionsReadRepository(repository OptionsReadRepository) Service {
	s.optionsRead = repository
	return s
}

func (s Service) WithMaintenanceReadRepository(repository MaintenanceReadRepository) Service {
	s.maintenanceRead = repository
	return s
}

func (s Service) WithMerchantReadRepository(repository MerchantReadRepository) Service {
	s.merchantRead = repository
	return s
}

func (s Service) WithMerchantWriteRepository(repository MerchantWriteRepository) Service {
	s.merchantWrite = repository
	return s
}

func (s Service) WithJumpGateReadRepository(repository JumpGateReadRepository) Service {
	s.jumpGateRead = repository
	return s
}

func (s Service) WithJumpGateWriteRepository(repository JumpGateWriteRepository) Service {
	s.jumpGateWrite = repository
	return s
}

func (s Service) WithEmpireReadRepository(repository EmpireReadRepository) Service {
	s.empireRead = repository
	return s
}

func (s Service) WithTechnologyReadRepository(repository TechnologyReadRepository) Service {
	s.technologyRead = repository
	return s
}

func (s Service) WithBuildingOptionsReadRepository(repository BuildingOptionsReadRepository) Service {
	s.buildingRead = repository
	return s
}

func (s Service) WithResearchOptionsReadRepository(repository ResearchOptionsReadRepository) Service {
	s.researchRead = repository
	return s
}

func (s Service) WithShipyardOptionsReadRepository(repository ShipyardOptionsReadRepository) Service {
	s.shipyardRead = repository
	return s
}

func (s Service) WithDefenseOptionsReadRepository(repository DefenseOptionsReadRepository) Service {
	s.defenseRead = repository
	return s
}

func (s Service) WithFleetOptionsReadRepository(repository FleetOptionsReadRepository) Service {
	s.fleetOptions = repository
	return s
}

func (s Service) WithPremiumWriteRepository(repository PremiumWriteRepository) Service {
	s.premiumWrite = repository
	return s
}

func (s Service) WithToolCallAuditor(auditor ToolCallAuditor) Service {
	s.auditor = auditor
	return s
}

func (s Service) WithTokenTTL(ttl time.Duration) Service {
	if ttl < 0 {
		ttl = defaultMCPTokenTTL
	}
	s.tokenTTL = ttl
	return s
}

func (s Service) Initialize(ctx context.Context) domainmcp.InitializeResult {
	_ = ctx
	return domainmcp.InitializeResult{
		ProtocolVersion: domainmcp.ProtocolVersion,
		Capabilities: domainmcp.Capabilities{
			Tools: &domainmcp.ToolsCapability{ListChanged: false},
		},
		ServerInfo: domainmcp.ServerInfo{
			Name:    "ogame-opensource",
			Title:   "OGame Open Source",
			Version: domainmcp.ProtocolVersion,
		},
	}
}

func (s Service) OAuthAuthorizationServerMetadata(ctx context.Context, issuer string) domainmcp.OAuthAuthorizationServerMetadata {
	_ = ctx
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	metadata := domainmcp.OAuthAuthorizationServerMetadata{
		Issuer:                            issuer,
		AuthorizationEndpoint:             issuer + "/oauth/authorize",
		TokenEndpoint:                     issuer + "/oauth/token",
		RevocationEndpoint:                issuer + "/oauth/revoke",
		RegistrationEndpoint:              issuer + "/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		RevocationEndpointAuthMethods:     []string{"none"},
		ScopesSupported: []string{
			"openid",
			"profile",
			domainmcp.ScopeRead,
			domainmcp.ScopeMessages,
			domainmcp.ScopeMessageWrite,
			domainmcp.ScopeNotesWrite,
			domainmcp.ScopeBuddyWrite,
			domainmcp.ScopeFleet,
			domainmcp.ScopeFleetWrite,
			domainmcp.ScopeQueueWrite,
			domainmcp.ScopeResourcesWrite,
			domainmcp.ScopePremiumWrite,
			domainmcp.ScopeMerchantWrite,
		},
	}
	if s.oidcSigner != nil {
		metadata.JWKSURI = issuer + "/.well-known/jwks.json"
		metadata.IDTokenSigningAlgValuesSupported = []string{s.oidcSigner.Algorithm()}
	}
	return metadata
}

func (s Service) OAuthProtectedResourceMetadata(ctx context.Context, resource string, issuer string) domainmcp.OAuthProtectedResourceMetadata {
	_ = ctx
	return domainmcp.OAuthProtectedResourceMetadata{
		Resource:             strings.TrimRight(strings.TrimSpace(resource), "/"),
		AuthorizationServers: []string{strings.TrimRight(strings.TrimSpace(issuer), "/")},
		ScopesSupported: []string{
			domainmcp.ScopeRead,
			domainmcp.ScopeMessages,
			domainmcp.ScopeMessageWrite,
			domainmcp.ScopeNotesWrite,
			domainmcp.ScopeBuddyWrite,
			domainmcp.ScopeFleet,
			domainmcp.ScopeFleetWrite,
			domainmcp.ScopeQueueWrite,
			domainmcp.ScopeResourcesWrite,
			domainmcp.ScopePremiumWrite,
			domainmcp.ScopeMerchantWrite,
		},
		BearerMethods: []string{"header"},
	}
}

func (s Service) RegisterOAuthClient(ctx context.Context, command OAuthClientRegistrationCommand) (OAuthClientRegistrationResult, error) {
	_ = ctx
	redirectURIs, err := normalizeOAuthClientRedirectURIs(command.RedirectURIs, s.oauthRedirects)
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	grantTypes, err := normalizeOAuthClientValues(command.GrantTypes, []string{"authorization_code"}, "grant_types")
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	responseTypes, err := normalizeOAuthClientValues(command.ResponseTypes, []string{"code"}, "response_types")
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	authMethod := strings.TrimSpace(command.TokenEndpointAuthMethod)
	if authMethod == "" {
		authMethod = "none"
	}
	if authMethod != "none" {
		return OAuthClientRegistrationResult{}, fmt.Errorf("%w: token_endpoint_auth_method must be none", ErrInvalidOAuthRequest)
	}
	scopes, err := normalizeOAuthScopes(command.Scope)
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	generator := s.tokenGenerator
	if generator == nil {
		generator = SecureTokenGenerator{}
	}
	secret, err := generator.NewMCPToken()
	if err != nil {
		return OAuthClientRegistrationResult{}, err
	}
	clientID := "ogmcp_client_" + strings.TrimPrefix(secret, "ogmcp_")
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	return OAuthClientRegistrationResult{
		ClientID:                clientID,
		ClientIDIssuedAt:        now().Unix(),
		ClientName:              truncateOAuthTokenName(strings.TrimSpace(command.ClientName), 64),
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		TokenEndpointAuthMethod: authMethod,
		Scope:                   strings.Join(scopes, " "),
	}, nil
}

func (s Service) OAuthJWKS(ctx context.Context) domainmcp.JSONWebKeySet {
	if s.oidcSigner == nil {
		return domainmcp.JSONWebKeySet{}
	}
	return s.oidcSigner.JWKS(ctx)
}

func (s Service) AuthorizeOAuth(ctx context.Context, command OAuthAuthorizeCommand) (OAuthAuthorizeResult, error) {
	if s.sessions == nil || s.oauthRepository == nil || s.codeGenerator == nil {
		return OAuthAuthorizeResult{}, errors.New("mcp oauth dependencies unavailable")
	}
	request, err := normalizeOAuthAuthorizeRequest(command, s.oauthRedirects)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	if !session.Authenticated {
		return OAuthAuthorizeResult{Authenticated: false, Issues: session.Issues}, nil
	}
	if !command.ConsentApproved {
		return OAuthAuthorizeResult{
			Authenticated:   true,
			RequiresConsent: true,
			ClientID:        request.ClientID,
			RedirectURI:     request.RedirectURI,
			Scopes:          request.Scopes,
		}, nil
	}
	code, err := s.codeGenerator.NewMCPOAuthCode()
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	now := s.now()
	_, err = s.oauthRepository.CreateMCPOAuthCode(ctx, domainmcp.OAuthAuthorizationCode{
		PlayerID:            session.Session.PlayerID,
		ClientID:            request.ClientID,
		RedirectURI:         request.RedirectURI,
		Resource:            request.Resource,
		Scopes:              request.Scopes,
		CodeHash:            HashToken(code),
		CodeChallenge:       request.CodeChallenge,
		CodeChallengeMethod: request.CodeChallengeMethod,
		CreatedAt:           now.Unix(),
		ExpiresAt:           now.Add(mcpOAuthCodeTTL).Unix(),
	})
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	redirectTo, err := oauthRedirectURI(request.RedirectURI, code, request.State)
	if err != nil {
		return OAuthAuthorizeResult{}, err
	}
	return OAuthAuthorizeResult{
		Authenticated: true,
		ClientID:      request.ClientID,
		RedirectURI:   request.RedirectURI,
		Scopes:        request.Scopes,
		RedirectTo:    redirectTo,
		Code:          code,
	}, nil
}

func (s Service) ExchangeOAuthCode(ctx context.Context, command OAuthTokenCommand) (OAuthTokenResult, error) {
	if s.oauthRepository == nil || s.tokenRepository == nil || s.tokenGenerator == nil {
		return OAuthTokenResult{}, errors.New("mcp oauth dependencies unavailable")
	}
	if strings.TrimSpace(command.GrantType) != "authorization_code" {
		return OAuthTokenResult{}, fmt.Errorf("%w: grant_type must be authorization_code", ErrInvalidOAuthRequest)
	}
	code := strings.TrimSpace(command.Code)
	if code == "" {
		return OAuthTokenResult{}, fmt.Errorf("%w: code is required", ErrInvalidOAuthRequest)
	}
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if !validOAuthRedirectURI(redirectURI, s.oauthRedirects) {
		return OAuthTokenResult{}, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
	}
	if !validOAuthResource(command.Resource, command.Issuer) {
		return OAuthTokenResult{}, fmt.Errorf("%w: resource is invalid", ErrInvalidOAuthRequest)
	}
	clientID := strings.TrimSpace(command.ClientID)
	if !validOAuthClientID(clientID) {
		return OAuthTokenResult{}, fmt.Errorf("%w: client_id is required", ErrInvalidOAuthRequest)
	}
	verifier := strings.TrimSpace(command.CodeVerifier)
	if !validPKCEValue(verifier) {
		return OAuthTokenResult{}, fmt.Errorf("%w: code_verifier is invalid", ErrInvalidOAuthRequest)
	}
	now := s.now().Unix()
	stored, err := s.oauthRepository.ConsumeMCPOAuthCode(ctx, HashToken(code), now)
	if err != nil {
		if errors.Is(err, domainmcp.ErrUnauthorized) {
			return OAuthTokenResult{}, ErrInvalidOAuthGrant
		}
		return OAuthTokenResult{}, err
	}
	resource := strings.TrimRight(strings.TrimSpace(command.Resource), "/")
	if stored.PlayerID <= 0 || stored.ClientID != clientID || stored.RedirectURI != redirectURI || stored.Resource != resource || stored.CodeChallengeMethod != "S256" || !pkceS256Matches(verifier, stored.CodeChallenge) {
		return OAuthTokenResult{}, ErrInvalidOAuthGrant
	}
	scopes, err := normalizeOAuthScopes(strings.Join(stored.Scopes, " "))
	if err != nil {
		return OAuthTokenResult{}, ErrInvalidOAuthGrant
	}
	secret, err := s.tokenGenerator.NewMCPToken()
	if err != nil {
		return OAuthTokenResult{}, err
	}
	token, err := s.tokenRepository.CreateMCPToken(ctx, domainmcp.Token{
		PlayerID:  stored.PlayerID,
		Name:      truncateOAuthTokenName("OAuth "+clientID, 64),
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: s.tokenExpiresAt(time.Unix(now, 0)),
	}, HashToken(secret))
	if err != nil {
		return OAuthTokenResult{}, err
	}
	token.PlayerID = 0
	idToken := ""
	if scopeAllowed(scopes, "openid") && s.oidcSigner != nil {
		idToken, err = s.oidcSigner.SignIDToken(ctx, domainmcp.IDTokenCommand{
			Issuer:   command.Issuer,
			Audience: clientID,
			PlayerID: stored.PlayerID,
			Scopes:   scopes,
		})
		if err != nil {
			return OAuthTokenResult{}, err
		}
	}
	return OAuthTokenResult{
		AccessToken: secret,
		TokenType:   "Bearer",
		Scope:       strings.Join(scopes, " "),
		IDToken:     idToken,
		ExpiresIn:   s.tokenExpiresIn(),
		Token:       token,
	}, nil
}

func (s Service) RevokeOAuthToken(ctx context.Context, command OAuthRevokeCommand) (OAuthRevokeResult, error) {
	if s.tokenRepository == nil {
		return OAuthRevokeResult{}, errors.New("mcp oauth token revocation dependencies unavailable")
	}
	token := strings.TrimSpace(command.Token)
	if token == "" {
		return OAuthRevokeResult{}, fmt.Errorf("%w: token is required", ErrInvalidOAuthRequest)
	}
	revoked, err := s.tokenRepository.RevokeMCPTokenByHash(ctx, HashToken(token), s.now().Unix())
	if err != nil {
		return OAuthRevokeResult{}, err
	}
	return OAuthRevokeResult{Revoked: revoked}, nil
}

func (s Service) ListTools(ctx context.Context, command domainmcp.ListToolsCommand) (domainmcp.ListToolsResult, error) {
	tools := []domainmcp.Tool{serverHealthTool()}
	if strings.TrimSpace(command.AccessToken) == "" {
		return domainmcp.ListToolsResult{Tools: tools}, nil
	}
	access, err := s.verify(ctx, command.AccessToken)
	if err != nil {
		return domainmcp.ListToolsResult{}, err
	}
	if access.HasScope(domainmcp.ScopeRead) {
		tools = append(tools, accessTool())
		if s.readRepository != nil {
			tools = append(tools, listPlanetsTool(), accountOverviewTool(), planetResourcesTool(), buildingQueueTool(), fleetMovementsTool())
		}
		if s.resourceRead != nil {
			tools = append(tools, resourceProductionOptionsTool())
		}
		if s.premiumRead != nil {
			tools = append(tools, officerStatusTool())
		}
		if s.searchRead != nil {
			tools = append(tools, searchGameTool())
		}
		if s.galaxyRead != nil {
			tools = append(tools, galaxySystemTool())
		}
		if s.statisticsRead != nil {
			tools = append(tools, statisticsTool())
		}
		if s.allianceRead != nil {
			tools = append(tools, allianceStatusTool())
		}
		if s.buddyRead != nil {
			tools = append(tools, buddyStatusTool())
		}
		if s.prangerRead != nil {
			tools = append(tools, prangerTool())
		}
		if s.notesRead != nil {
			tools = append(tools, notesTool())
		}
		if s.optionsRead != nil {
			tools = append(tools, optionsTool())
		}
		if s.maintenanceRead != nil {
			tools = append(tools, maintenanceTool())
		}
		if s.merchantRead != nil {
			tools = append(tools, merchantStatusTool())
		}
		if s.jumpGateRead != nil {
			tools = append(tools, jumpGateStatusTool())
		}
		if s.empireRead != nil {
			tools = append(tools, empireOverviewTool())
		}
		if s.technologyRead != nil {
			tools = append(tools, technologyTreeTool())
		}
		if s.buildingRead != nil {
			tools = append(tools, buildingOptionsTool())
		}
		if s.researchRead != nil {
			tools = append(tools, researchOptionsTool())
		}
		if s.shipyardRead != nil {
			tools = append(tools, shipyardOptionsTool())
		}
		if s.defenseRead != nil {
			tools = append(tools, defenseOptionsTool())
		}
		if s.fleetOptions != nil {
			tools = append(tools, fleetOptionsTool())
		}
	}
	if access.HasScope(domainmcp.ScopeMessages) {
		if s.readRepository != nil {
			tools = append(tools, listMessagesTool(), getMessageTool())
		}
		if s.reportRead != nil {
			tools = append(tools, getReportTool())
		}
	}
	if access.HasScope(domainmcp.ScopeMessageWrite) && s.writeRepository != nil {
		tools = append(tools, sendMessageTool(), deleteMessagesTool(), reportMessageTool())
	}
	if access.HasScope(domainmcp.ScopeNotesWrite) && s.notesWrite != nil {
		tools = append(tools, createNoteTool(), updateNoteTool(), deleteNotesTool())
	}
	if access.HasScope(domainmcp.ScopeBuddyWrite) && s.buddyWrite != nil {
		tools = append(tools, mutateBuddyTool())
	}
	if access.HasScope(domainmcp.ScopeFleetWrite) && s.fleetWrite != nil {
		tools = append(tools, validateFleetDispatchTool(), dispatchFleetTool(), recallFleetTool())
	}
	if access.HasScope(domainmcp.ScopeFleetWrite) && s.phalanxWrite != nil {
		tools = append(tools, scanPhalanxTool())
	}
	if access.HasScope(domainmcp.ScopeFleetWrite) && s.jumpGateWrite != nil {
		tools = append(tools, jumpGateTool())
	}
	if access.HasScope(domainmcp.ScopeQueueWrite) && s.queueWrite != nil {
		tools = append(tools, cancelBuildingQueueTool(), cancelResearchQueueTool(), enqueueShipyardOrderTool())
	}
	if access.HasScope(domainmcp.ScopeResourcesWrite) && s.resourceWrite != nil {
		tools = append(tools, updateResourceProductionTool())
	}
	if access.HasScope(domainmcp.ScopePremiumWrite) && s.premiumWrite != nil {
		tools = append(tools, recruitOfficerTool())
	}
	if access.HasScope(domainmcp.ScopeMerchantWrite) && s.merchantWrite != nil {
		tools = append(tools, mutateMerchantTool())
	}
	return domainmcp.ListToolsResult{Tools: tools}, nil
}

func (s Service) CallTool(ctx context.Context, command domainmcp.CallToolCommand) (result domainmcp.ToolCallResult, err error) {
	started := s.now()
	audit := domainmcp.ToolCallAudit{ToolName: command.Name, At: started.Unix()}
	defer func() {
		audit.DurationMS = s.now().Sub(started).Milliseconds()
		if err != nil {
			audit.Error = err.Error()
		}
		s.auditToolCall(ctx, audit)
	}()

	switch command.Name {
	case "get_server_health":
		audit.Authorized = true
		return s.callServerHealth(ctx)
	case "get_mcp_access":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return callMCPAccess(access), nil
	case "list_planets":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callListPlanets(ctx, access)
	case "get_account_overview":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callAccountOverview(ctx, access)
	case "get_planet_resources":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callPlanetResources(ctx, access, command.Arguments)
	case "get_resource_production_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callResourceProductionOptions(ctx, access, command.Arguments)
	case "get_building_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callBuildingQueue(ctx, access, command.Arguments)
	case "get_fleet_movements":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callFleetMovements(ctx, access)
	case "get_fleet_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callFleetOptions(ctx, access, command.Arguments)
	case "get_officer_status":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callOfficerStatus(ctx, access, command.Arguments)
	case "search_game":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callSearchGame(ctx, access, command.Arguments)
	case "get_galaxy_system":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callGalaxySystem(ctx, access, command.Arguments)
	case "get_statistics":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callStatistics(ctx, access, command.Arguments)
	case "get_alliance_status":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callAllianceStatus(ctx, access, command.Arguments)
	case "get_buddy_status":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callBuddyStatus(ctx, access, command.Arguments)
	case "get_pranger":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callPranger(ctx, access, command.Arguments)
	case "get_notes":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callNotes(ctx, access, command.Arguments)
	case "create_note":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeNotesWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callCreateNote(ctx, access, command.Arguments)
	case "update_note":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeNotesWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callUpdateNote(ctx, access, command.Arguments)
	case "delete_notes":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeNotesWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDeleteNotes(ctx, access, command.Arguments)
	case "mutate_buddy":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeBuddyWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callMutateBuddy(ctx, access, command.Arguments)
	case "get_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callOptions(ctx, access, command.Arguments)
	case "get_maintenance":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callMaintenance(ctx, access)
	case "get_merchant_status":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callMerchantStatus(ctx, access, command.Arguments)
	case "mutate_merchant":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMerchantWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callMutateMerchant(ctx, access, command.Arguments)
	case "get_jump_gate_status":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callJumpGateStatus(ctx, access, command.Arguments)
	case "get_empire_overview":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callEmpireOverview(ctx, access, command.Arguments)
	case "get_technology_tree":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callTechnologyTree(ctx, access, command.Arguments)
	case "get_building_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callBuildingOptions(ctx, access, command.Arguments)
	case "get_research_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callResearchOptions(ctx, access, command.Arguments)
	case "get_shipyard_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callShipyardOptions(ctx, access, command.Arguments)
	case "get_defense_options":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeRead)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDefenseOptions(ctx, access, command.Arguments)
	case "list_messages":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessages)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callListMessages(ctx, access, command.Arguments)
	case "get_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessages)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callGetMessage(ctx, access, command.Arguments)
	case "get_report":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessages)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callGetReport(ctx, access, command.Arguments)
	case "send_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callSendMessage(ctx, access, command.Arguments)
	case "delete_messages":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDeleteMessages(ctx, access, command.Arguments)
	case "report_message":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeMessageWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callReportMessage(ctx, access, command.Arguments)
	case "recall_fleet":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callRecallFleet(ctx, access, command.Arguments)
	case "validate_fleet_dispatch":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callValidateFleetDispatch(ctx, access, command.Arguments)
	case "dispatch_fleet":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callDispatchFleet(ctx, access, command.Arguments)
	case "scan_phalanx":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callScanPhalanx(ctx, access, command.Arguments)
	case "jump_gate":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeFleetWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callJumpGate(ctx, access, command.Arguments)
	case "cancel_building_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callCancelBuildingQueue(ctx, access, command.Arguments)
	case "cancel_research_queue":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callCancelResearchQueue(ctx, access, command.Arguments)
	case "enqueue_shipyard_order":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeQueueWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callEnqueueShipyardOrder(ctx, access, command.Arguments)
	case "update_resource_production":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopeResourcesWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callUpdateResourceProduction(ctx, access, command.Arguments)
	case "recruit_officer":
		access, err := s.authorize(ctx, command.AccessToken, domainmcp.ScopePremiumWrite)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		audit.PlayerID = access.PlayerID
		audit.Scopes = access.Scopes
		audit.Authorized = true
		return s.callRecruitOfficer(ctx, access, command.Arguments)
	default:
		return domainmcp.ToolCallResult{}, domainmcp.ErrToolNotFound
	}
}

func (s Service) ListTokens(ctx context.Context, command TokenManagementCommand) (TokenListResult, error) {
	if s.sessions == nil || s.tokenRepository == nil {
		return TokenListResult{}, errors.New("mcp token management dependencies unavailable")
	}
	session, err := s.authenticateSession(ctx, command)
	if err != nil {
		return TokenListResult{}, err
	}
	if !session.Authenticated {
		return TokenListResult{Authenticated: false, Issues: session.Issues}, nil
	}
	tokens, err := s.tokenRepository.ListMCPTokens(ctx, session.Session.PlayerID)
	if err != nil {
		return TokenListResult{}, err
	}
	return TokenListResult{Authenticated: true, Tokens: tokens}, nil
}

func (s Service) CreateToken(ctx context.Context, command CreateTokenCommand) (TokenCreationResult, error) {
	if s.sessions == nil || s.tokenRepository == nil || s.tokenGenerator == nil {
		return TokenCreationResult{}, errors.New("mcp token management dependencies unavailable")
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return TokenCreationResult{}, err
	}
	if !session.Authenticated {
		return TokenCreationResult{Authenticated: false, Issues: session.Issues}, nil
	}
	name, scopes, err := normalizeTokenRequest(command.Name, command.Scopes)
	if err != nil {
		return TokenCreationResult{}, err
	}
	secret, err := s.tokenGenerator.NewMCPToken()
	if err != nil {
		return TokenCreationResult{}, err
	}
	now := s.now().Unix()
	token, err := s.tokenRepository.CreateMCPToken(ctx, domainmcp.Token{
		PlayerID:  session.Session.PlayerID,
		Name:      name,
		Scopes:    scopes,
		CreatedAt: now,
		ExpiresAt: s.tokenExpiresAt(time.Unix(now, 0)),
	}, HashToken(secret))
	if err != nil {
		return TokenCreationResult{}, err
	}
	token.PlayerID = 0
	return TokenCreationResult{
		Authenticated: true,
		Creation:      domainmcp.TokenCreation{Token: token, Secret: secret},
	}, nil
}

func (s Service) RevokeToken(ctx context.Context, command RevokeTokenCommand) (TokenRevokeResult, error) {
	if s.sessions == nil || s.tokenRepository == nil {
		return TokenRevokeResult{}, errors.New("mcp token management dependencies unavailable")
	}
	if command.TokenID <= 0 {
		return TokenRevokeResult{}, fmt.Errorf("%w: token id is required", ErrInvalidTokenRequest)
	}
	session, err := s.authenticateSession(ctx, command.TokenManagementCommand)
	if err != nil {
		return TokenRevokeResult{}, err
	}
	if !session.Authenticated {
		return TokenRevokeResult{Authenticated: false, Issues: session.Issues}, nil
	}
	revoked, err := s.tokenRepository.RevokeMCPToken(ctx, session.Session.PlayerID, command.TokenID, s.now().Unix())
	if err != nil {
		return TokenRevokeResult{}, err
	}
	return TokenRevokeResult{Authenticated: true, Revoked: revoked}, nil
}

func (s Service) tokenExpiresAt(now time.Time) int64 {
	if s.tokenTTL <= 0 {
		return 0
	}
	return now.Add(s.tokenTTL).Unix()
}

func (s Service) tokenExpiresIn() int64 {
	if s.tokenTTL <= 0 {
		return 0
	}
	return int64(s.tokenTTL.Seconds())
}

func (s Service) callServerHealth(ctx context.Context) (domainmcp.ToolCallResult, error) {
	health := s.health.Get(ctx)
	structured := map[string]any{
		"status":            health.Status,
		"service":           health.Service,
		"environment":       health.Environment,
		"runtime":           health.Runtime,
		"goTarget":          health.Targets.Go,
		"bunTarget":         health.Targets.Bun,
		"reactTarget":       health.Targets.React,
		"staticReady":       health.StaticReady,
		"legacyAssetsReady": health.LegacyAssetsReady,
		"legacyBaseUrl":     health.LegacyBaseURL,
	}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callListPlanets(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planets, err := s.readRepository.ListMCPPlanets(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{
		"playerId": access.PlayerID,
		"count":    len(planets),
		"planets":  planets,
	}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callAccountOverview(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	overview, err := s.readRepository.GetMCPAccountOverview(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"overview": overview}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callPlanetResources(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	resources, err := s.readRepository.GetMCPPlanetResources(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"resources": resources}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callResourceProductionOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.resourceRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp resource production read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	options, err := s.resourceRead.GetMCPResourceProductionOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"resourceProductionOptions": options}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callBuildingQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	queue, err := s.readRepository.GetMCPBuildingQueue(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"buildingQueue": queue}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callFleetMovements(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	movements, err := s.readRepository.GetMCPFleetMovements(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"fleetMovements": movements}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callFleetOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetOptions == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet options read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	options, err := s.fleetOptions.GetMCPFleetOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"fleetOptions": options}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callOfficerStatus(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.premiumRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp premium read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	status, err := s.premiumRead.GetMCPOfficerStatus(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"officerStatus": status}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callSearchGame(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.searchRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp search read repository unavailable")
	}
	command, err := mcpSearchCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.searchRead.SearchMCP(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"search": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callGalaxySystem(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.galaxyRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp galaxy read repository unavailable")
	}
	command, err := mcpGalaxySystemCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.galaxyRead.GetMCPGalaxySystem(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"galaxySystem": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callStatistics(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.statisticsRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp statistics read repository unavailable")
	}
	command, err := mcpStatisticsCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.statisticsRead.GetMCPStatistics(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"statistics": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callAllianceStatus(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.allianceRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp alliance read repository unavailable")
	}
	command, err := mcpAllianceStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	status, err := s.allianceRead.GetMCPAllianceStatus(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"allianceStatus": status}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callBuddyStatus(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.buddyRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp buddy read repository unavailable")
	}
	command, err := mcpBuddyStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	status, err := s.buddyRead.GetMCPBuddyStatus(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"buddyStatus": status}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callMutateBuddy(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.buddyWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp buddy write repository unavailable")
	}
	command, err := mcpBuddyMutationCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpBuddyMutationConfirmation(command)
	var result domainmcp.BuddyMutationResult
	if command.DryRun {
		result, err = s.buddyWrite.PreviewMCPBuddyMutation(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.buddyWrite.MutateMCPBuddy(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
		result.Executed = true
	}
	structured := map[string]any{"buddyMutation": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callPranger(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.prangerRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp pranger read repository unavailable")
	}
	command, err := mcpPrangerCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	pranger, err := s.prangerRead.GetMCPPranger(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"pranger": pranger}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callNotes(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.notesRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp notes read repository unavailable")
	}
	command, err := mcpNotesStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	notes, err := s.notesRead.GetMCPNotes(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"notes": notes}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callCreateNote(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.notesWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp notes write repository unavailable")
	}
	command, err := mcpNoteMutationCommand(arguments, false, false)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpCreateNoteConfirmation(command)
	var result domainmcp.NoteMutationResult
	if command.DryRun {
		result, err = s.notesWrite.PreviewMCPCreateNote(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		result.RequiresConfirmation = true
		result.Confirmation = confirmation
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.notesWrite.CreateMCPNote(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
		result.Executed = true
	}
	structured := map[string]any{"createNote": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callUpdateNote(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.notesWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp notes write repository unavailable")
	}
	command, err := mcpNoteMutationCommand(arguments, true, false)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpUpdateNoteConfirmation(command)
	var result domainmcp.NoteMutationResult
	if command.DryRun {
		result, err = s.notesWrite.PreviewMCPUpdateNote(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		result.RequiresConfirmation = true
		result.Confirmation = confirmation
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.notesWrite.UpdateMCPNote(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
		result.Executed = true
	}
	structured := map[string]any{"updateNote": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDeleteNotes(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.notesWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp notes write repository unavailable")
	}
	command, err := mcpNoteMutationCommand(arguments, false, true)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpDeleteNotesConfirmation(command)
	var result domainmcp.NoteMutationResult
	if command.DryRun {
		result, err = s.notesWrite.PreviewMCPDeleteNotes(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.DeleteCount > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.notesWrite.DeleteMCPNotes(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
		result.Executed = true
	}
	structured := map[string]any{"deleteNotes": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.optionsRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp options read repository unavailable")
	}
	command, err := mcpOptionsStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	options, err := s.optionsRead.GetMCPOptions(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"options": options}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callMaintenance(ctx context.Context, access domainmcp.Access) (domainmcp.ToolCallResult, error) {
	if s.maintenanceRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp maintenance read repository unavailable")
	}
	maintenance, err := s.maintenanceRead.GetMCPMaintenance(ctx, access.PlayerID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"maintenance": maintenance}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callMerchantStatus(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.merchantRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp merchant read repository unavailable")
	}
	command, err := mcpMerchantStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	status, err := s.merchantRead.GetMCPMerchantStatus(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"merchantStatus": status}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callMutateMerchant(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.merchantWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp merchant write repository unavailable")
	}
	command, err := mcpMerchantMutationCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpMerchantMutationConfirmation(command)
	var result domainmcp.MerchantMutationResult
	if command.DryRun {
		result, err = s.merchantWrite.PreviewMCPMutateMerchant(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.merchantWrite.MutateMCPMerchant(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"merchantMutation": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callJumpGateStatus(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.jumpGateRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp jump gate read repository unavailable")
	}
	command, err := mcpJumpGateStatusCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	status, err := s.jumpGateRead.GetMCPJumpGateStatus(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"jumpGateStatus": status}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callEmpireOverview(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.empireRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp empire read repository unavailable")
	}
	command, err := mcpEmpireCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.empireRead.GetMCPEmpire(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"empire": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callTechnologyTree(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.technologyRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp technology read repository unavailable")
	}
	command, err := mcpTechnologyCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.technologyRead.GetMCPTechnology(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"technology": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callBuildingOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.buildingRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp building read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.buildingRead.GetMCPBuildingOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"buildingOptions": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callResearchOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.researchRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp research read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.researchRead.GetMCPResearchOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"researchOptions": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callShipyardOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.shipyardRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp shipyard read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.shipyardRead.GetMCPShipyardOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"shipyardOptions": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDefenseOptions(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.defenseRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp defense read repository unavailable")
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.defenseRead.GetMCPDefenseOptions(ctx, access.PlayerID, planetID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"defenseOptions": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callListMessages(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	query, err := mcpMessageQuery(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	messages, err := s.readRepository.ListMCPMessages(ctx, access.PlayerID, query)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"messages": messages}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callGetMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.readRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp read repository unavailable")
	}
	messageID, err := optionalNonNegativeIntArgument(arguments, "messageId")
	if err != nil || messageID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	message, err := s.readRepository.GetMCPMessage(ctx, access.PlayerID, messageID)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"message": message}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callGetReport(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.reportRead == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp report repository unavailable")
	}
	reportID, err := optionalNonNegativeIntArgument(arguments, "reportId")
	if err != nil || reportID <= 0 {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	report, err := s.reportRead.GetMCPReport(ctx, access.PlayerID, domainmcp.ReportCommand{ReportID: reportID})
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	structured := map[string]any{"report": report}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callSendMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpSendMessageCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpSendMessageConfirmation(command)
	var result domainmcp.SendMessageResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPSendMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.SendMCPMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"sendMessage": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDeleteMessages(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpDeleteMessagesCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpDeleteMessagesConfirmation(command)
	var result domainmcp.DeleteMessagesResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPDeleteMessages(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.DeleteCount > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.DeleteMCPMessages(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"deleteMessages": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callReportMessage(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.writeRepository == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp write repository unavailable")
	}
	command, err := mcpReportMessageCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpReportMessageConfirmation(command)
	var result domainmcp.ReportMessageResult
	if command.DryRun {
		result, err = s.writeRepository.PreviewMCPReportMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Reportable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.writeRepository.ReportMCPMessage(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"reportMessage": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callValidateFleetDispatch(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpDispatchFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result, err := s.fleetWrite.PreviewMCPDispatchFleet(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.DryRun = true
	if result.Ready && result.Issue == nil {
		result.RequiresConfirmation = true
		result.Confirmation = mcpDispatchFleetConfirmation(command)
	}
	structured := map[string]any{"fleetDispatchValidation": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callDispatchFleet(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpDispatchFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpDispatchFleetConfirmation(command)
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	if strings.TrimSpace(confirm) != confirmation {
		return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
	}
	result, err := s.fleetWrite.DispatchMCPFleet(ctx, access.PlayerID, command)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	result.DryRun = false
	result.RequiresConfirmation = false
	result.Confirmation = confirmation
	structured := map[string]any{"fleetDispatch": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callRecallFleet(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.fleetWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp fleet write repository unavailable")
	}
	command, err := mcpRecallFleetCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpRecallFleetConfirmation(command)
	var result domainmcp.RecallFleetResult
	if command.DryRun {
		result, err = s.fleetWrite.PreviewMCPRecallFleet(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Recallable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.fleetWrite.RecallMCPFleet(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"recallFleet": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callScanPhalanx(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.phalanxWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp phalanx repository unavailable")
	}
	command, err := mcpPhalanxScanCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpPhalanxScanConfirmation(command)
	var result domainmcp.PhalanxScanResult
	if command.DryRun {
		result, err = s.phalanxWrite.PreviewMCPPhalanxScan(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.phalanxWrite.ScanMCPPhalanx(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"phalanxScan": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callJumpGate(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.jumpGateWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp jump gate write repository unavailable")
	}
	command, err := mcpJumpGateCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpJumpGateConfirmation(command)
	var result domainmcp.JumpGateResult
	if command.DryRun {
		result, err = s.jumpGateWrite.PreviewMCPJumpGate(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.jumpGateWrite.JumpMCPJumpGate(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"jumpGate": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callCancelBuildingQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpCancelBuildingQueueCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpCancelBuildingQueueConfirmation(command)
	var result domainmcp.CancelBuildingQueueResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPCancelBuildingQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Cancelable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.CancelMCPBuildingQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"cancelBuildingQueue": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callCancelResearchQueue(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpCancelResearchQueueCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpCancelResearchQueueConfirmation()
	var result domainmcp.CancelResearchQueueResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPCancelResearchQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Cancelable && result.Issue == nil {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.CancelMCPResearchQueue(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"cancelResearchQueue": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callEnqueueShipyardOrder(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.queueWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp queue write repository unavailable")
	}
	command, err := mcpEnqueueShipyardOrderCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpEnqueueShipyardOrderConfirmation(command)
	var result domainmcp.EnqueueShipyardOrderResult
	if command.DryRun {
		result, err = s.queueWrite.PreviewMCPEnqueueShipyardOrder(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && result.Amount > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.queueWrite.EnqueueMCPShipyardOrder(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"enqueueShipyardOrder": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callUpdateResourceProduction(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.resourceWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp resource write repository unavailable")
	}
	command, err := mcpUpdateResourceProductionCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpUpdateResourceProductionConfirmation(command)
	var result domainmcp.UpdateResourceProductionResult
	if command.DryRun {
		result, err = s.resourceWrite.PreviewMCPUpdateResourceProduction(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && len(result.Settings) > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.resourceWrite.UpdateMCPResourceProduction(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"updateResourceProduction": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) callRecruitOfficer(ctx context.Context, access domainmcp.Access, arguments map[string]any) (domainmcp.ToolCallResult, error) {
	if s.premiumWrite == nil {
		return domainmcp.ToolCallResult{}, errors.New("mcp premium write repository unavailable")
	}
	command, err := mcpRecruitOfficerCommand(arguments)
	if err != nil {
		return domainmcp.ToolCallResult{}, err
	}
	confirmation := mcpRecruitOfficerConfirmation(command)
	var result domainmcp.RecruitOfficerResult
	if command.DryRun {
		result, err = s.premiumWrite.PreviewMCPRecruitOfficer(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = true
		result.Executed = false
		if result.Issue == nil && result.OfficerID > 0 {
			result.RequiresConfirmation = true
			result.Confirmation = confirmation
		}
	} else {
		if strings.TrimSpace(command.Confirm) != confirmation {
			return domainmcp.ToolCallResult{}, domainmcp.ErrInvalidParams
		}
		result, err = s.premiumWrite.RecruitMCPOfficer(ctx, access.PlayerID, command)
		if err != nil {
			return domainmcp.ToolCallResult{}, err
		}
		result.DryRun = false
		result.RequiresConfirmation = false
		result.Confirmation = confirmation
	}
	structured := map[string]any{"recruitOfficer": result}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}, nil
}

func (s Service) authenticateSession(ctx context.Context, command TokenManagementCommand) (domainpublicsite.SessionAuthentication, error) {
	return s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
}

func (s Service) auditToolCall(ctx context.Context, audit domainmcp.ToolCallAudit) {
	if s.auditor == nil {
		return
	}
	s.auditor.RecordMCPToolCall(ctx, audit)
}

func (s Service) authorize(ctx context.Context, token string, scope string) (domainmcp.Access, error) {
	access, err := s.verify(ctx, token)
	if err != nil {
		return domainmcp.Access{}, err
	}
	if !access.HasScope(scope) {
		return domainmcp.Access{}, domainmcp.ErrForbidden
	}
	return access, nil
}

func HashToken(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func normalizeTokenRequest(name string, requested []string) (string, []string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "MCP token"
	}
	if len(name) > 64 {
		return "", nil, fmt.Errorf("%w: name is too long", ErrInvalidTokenRequest)
	}
	scopes := normalizeUserScopes(requested)
	if len(scopes) == 0 {
		scopes = []string{domainmcp.ScopeRead}
	}
	for _, scope := range scopes {
		if !userScopeAllowed(scope) {
			return "", nil, fmt.Errorf("%w: scope %q is not available for user tokens", ErrInvalidTokenRequest, scope)
		}
	}
	return name, scopes, nil
}

func normalizeUserScopes(requested []string) []string {
	seen := map[string]bool{}
	scopes := make([]string, 0, len(requested))
	for _, raw := range requested {
		scope := strings.TrimSpace(raw)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	return scopes
}

func userScopeAllowed(scope string) bool {
	switch scope {
	case domainmcp.ScopeRead, domainmcp.ScopeMessages, domainmcp.ScopeMessageWrite, domainmcp.ScopeNotesWrite, domainmcp.ScopeBuddyWrite, domainmcp.ScopeFleet, domainmcp.ScopeFleetWrite, domainmcp.ScopeQueueWrite, domainmcp.ScopeResourcesWrite, domainmcp.ScopePremiumWrite, domainmcp.ScopeMerchantWrite:
		return true
	default:
		return false
	}
}

type normalizedOAuthAuthorizeRequest struct {
	ClientID            string
	RedirectURI         string
	Resource            string
	State               string
	Scopes              []string
	CodeChallenge       string
	CodeChallengeMethod string
}

func normalizeOAuthAuthorizeRequest(command OAuthAuthorizeCommand, allowedRedirectURIs []string) (normalizedOAuthAuthorizeRequest, error) {
	if strings.TrimSpace(command.ResponseType) != "code" {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: response_type must be code", ErrInvalidOAuthRequest)
	}
	clientID := strings.TrimSpace(command.ClientID)
	if !validOAuthClientID(clientID) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: client_id is required", ErrInvalidOAuthRequest)
	}
	redirectURI := strings.TrimSpace(command.RedirectURI)
	if !validOAuthRedirectURI(redirectURI, allowedRedirectURIs) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
	}
	if !validOAuthResource(command.Resource, command.Issuer) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: resource is invalid", ErrInvalidOAuthRequest)
	}
	method := strings.TrimSpace(command.CodeChallengeMethod)
	if method != "S256" {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: code_challenge_method must be S256", ErrInvalidOAuthRequest)
	}
	challenge := strings.TrimSpace(command.CodeChallenge)
	if !validPKCEValue(challenge) {
		return normalizedOAuthAuthorizeRequest{}, fmt.Errorf("%w: code_challenge is invalid", ErrInvalidOAuthRequest)
	}
	scopes, err := normalizeOAuthScopes(command.Scope)
	if err != nil {
		return normalizedOAuthAuthorizeRequest{}, err
	}
	return normalizedOAuthAuthorizeRequest{
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Resource:            strings.TrimRight(strings.TrimSpace(command.Resource), "/"),
		State:               strings.TrimSpace(command.State),
		Scopes:              scopes,
		CodeChallenge:       challenge,
		CodeChallengeMethod: method,
	}, nil
}

func normalizeOAuthScopes(raw string) ([]string, error) {
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		parts = []string{domainmcp.ScopeRead}
	}
	seen := map[string]bool{}
	scopes := make([]string, 0, len(parts))
	for _, scope := range parts {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		if !oauthScopeAllowed(scope) {
			return nil, fmt.Errorf("%w: scope %q is not available", ErrInvalidOAuthRequest, scope)
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		scopes = append(scopes, domainmcp.ScopeRead)
	}
	return scopes, nil
}

func normalizeOAuthClientRedirectURIs(raw []string, allowedRedirectURIs []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: redirect_uris are required", ErrInvalidOAuthRequest)
	}
	redirects := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		parsed := strings.TrimSpace(value)
		if !validOAuthRedirectURI(parsed, allowedRedirectURIs) {
			return nil, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
		}
		normalized, ok := normalizedOAuthResource(parsed)
		if !ok {
			return nil, fmt.Errorf("%w: redirect_uri is invalid", ErrInvalidOAuthRequest)
		}
		if !seen[normalized] {
			seen[normalized] = true
			redirects = append(redirects, normalized)
		}
	}
	return redirects, nil
}

func normalizeOAuthClientValues(raw []string, allowed []string, field string) ([]string, error) {
	if len(raw) == 0 {
		return append([]string(nil), allowed...), nil
	}
	allowedSet := map[string]bool{}
	for _, value := range allowed {
		allowedSet[value] = true
	}
	values := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		value = strings.TrimSpace(value)
		if !allowedSet[value] {
			return nil, fmt.Errorf("%w: %s is unsupported", ErrInvalidOAuthRequest, field)
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%w: %s is required", ErrInvalidOAuthRequest, field)
	}
	return values, nil
}

func oauthScopeAllowed(scope string) bool {
	return scope == "openid" || scope == "profile" || userScopeAllowed(scope)
}

func scopeAllowed(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

func validOAuthClientID(clientID string) bool {
	return clientID != "" && len(clientID) <= 128 && !strings.ContainsAny(clientID, " \t\r\n")
}

func validOAuthRedirectURI(raw string, allowedRedirectURIs []string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return false
	}
	if oauthRedirectAllowedByConfig(parsed.String(), allowedRedirectURIs) {
		return true
	}
	switch parsed.Scheme {
	case "http", "https":
		return isLoopbackHost(parsed.Hostname())
	default:
		return false
	}
}

func validOAuthResource(raw string, issuer string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	resource, ok := normalizedOAuthResource(raw)
	if !ok {
		return false
	}
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	for _, candidate := range []string{issuer, issuer + "/mcp"} {
		normalized, ok := normalizedOAuthResource(candidate)
		if ok && resource == normalized {
			return true
		}
	}
	return false
}

func normalizedOAuthResource(raw string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
		return "", false
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), true
}

func normalizeAllowedOAuthRedirectURIs(raw []string) []string {
	allowed := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, value := range raw {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.Fragment != "" {
			continue
		}
		normalized := parsed.String()
		if !seen[normalized] {
			seen[normalized] = true
			allowed = append(allowed, normalized)
		}
	}
	return allowed
}

func ParseOAuthRedirectURIs(raw string) []string {
	return normalizeAllowedOAuthRedirectURIs(strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	}))
}

func oauthRedirectAllowedByConfig(raw string, allowedRedirectURIs []string) bool {
	for _, allowed := range allowedRedirectURIs {
		if raw == allowed {
			return true
		}
	}
	return false
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, ch := range value {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '.' || ch == '_' || ch == '~' {
			continue
		}
		return false
	}
	return true
}

func pkceS256Matches(verifier string, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return computed == strings.TrimSpace(challenge)
}

func oauthRedirectURI(raw string, code string, state string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("code", code)
	if strings.TrimSpace(state) != "" {
		query.Set("state", strings.TrimSpace(state))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func truncateOAuthTokenName(name string, limit int) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}

func optionalNonNegativeIntArgument(arguments map[string]any, name string) (int, error) {
	if arguments == nil {
		return 0, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return 0, nil
	}
	return nonNegativeIntValue(raw, name)
}

func nonNegativeIntValue(raw any, name string) (int, error) {
	maxIntValue := int64(^uint(0) >> 1)
	var value int64
	switch typed := raw.(type) {
	case int:
		value = int64(typed)
	case int64:
		value = typed
	case float64:
		if math.Trunc(typed) != typed {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		if typed < 0 || typed > float64(maxIntValue) {
			return 0, fmt.Errorf("%w: %s must be a non-negative integer", domainmcp.ErrInvalidParams, name)
		}
		value = int64(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		value = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
		}
		value = parsed
	default:
		return 0, fmt.Errorf("%w: %s must be an integer", domainmcp.ErrInvalidParams, name)
	}
	if value < 0 || value > maxIntValue {
		return 0, fmt.Errorf("%w: %s must be a non-negative integer", domainmcp.ErrInvalidParams, name)
	}
	return int(value), nil
}

func mcpSendMessageCommand(arguments map[string]any) (domainmcp.SendMessageCommand, error) {
	targetPlayerID, err := optionalNonNegativeIntArgument(arguments, "targetPlayerId")
	if err != nil || targetPlayerID <= 0 {
		return domainmcp.SendMessageCommand{}, domainmcp.ErrInvalidParams
	}
	subject, err := optionalStringArgument(arguments, "subject")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	text, err := optionalStringArgument(arguments, "text")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.SendMessageCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.SendMessageCommand{}, err
	}
	return domainmcp.SendMessageCommand{
		TargetPlayerID: targetPlayerID,
		Subject:        subject,
		Text:           text,
		DryRun:         dryRun,
		Confirm:        confirm,
	}, nil
}

func mcpSendMessageConfirmation(command domainmcp.SendMessageCommand) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s\n%s", command.TargetPlayerID, command.Subject, command.Text)))
	return fmt.Sprintf("send_message:%d:%s", command.TargetPlayerID, hex.EncodeToString(sum[:])[:12])
}

func mcpDeleteMessagesCommand(arguments map[string]any) (domainmcp.DeleteMessagesCommand, error) {
	messageIDs, err := positiveIntSliceArgument(arguments, "messageIds")
	if err != nil || len(messageIDs) == 0 {
		return domainmcp.DeleteMessagesCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.DeleteMessagesCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.DeleteMessagesCommand{}, err
	}
	return domainmcp.DeleteMessagesCommand{MessageIDs: messageIDs, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpDeleteMessagesConfirmation(command domainmcp.DeleteMessagesCommand) string {
	ids := make([]string, 0, len(command.MessageIDs))
	for _, id := range command.MessageIDs {
		ids = append(ids, strconv.Itoa(id))
	}
	payload := strings.Join(ids, ",")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("delete_messages:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpReportMessageCommand(arguments map[string]any) (domainmcp.ReportMessageCommand, error) {
	messageID, err := optionalNonNegativeIntArgument(arguments, "messageId")
	if err != nil || messageID <= 0 {
		return domainmcp.ReportMessageCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.ReportMessageCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.ReportMessageCommand{}, err
	}
	return domainmcp.ReportMessageCommand{MessageID: messageID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpReportMessageConfirmation(command domainmcp.ReportMessageCommand) string {
	payload := strconv.Itoa(command.MessageID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("report_message:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpDispatchFleetCommand(arguments map[string]any) (domainmcp.DispatchFleetCommand, error) {
	ships, err := intObjectArgument(arguments, "ships")
	if err != nil || len(ships) == 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetGalaxy, err := optionalNonNegativeIntArgument(arguments, "targetGalaxy")
	if err != nil || targetGalaxy <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetSystem, err := optionalNonNegativeIntArgument(arguments, "targetSystem")
	if err != nil || targetSystem <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetPosition, err := optionalNonNegativeIntArgument(arguments, "targetPosition")
	if err != nil || targetPosition <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	targetType, err := optionalNonNegativeIntArgument(arguments, "targetType")
	if err != nil || targetType <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	mission, err := optionalNonNegativeIntArgument(arguments, "mission")
	if err != nil || mission <= 0 {
		return domainmcp.DispatchFleetCommand{}, domainmcp.ErrInvalidParams
	}
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	speed, err := optionalNonNegativeIntArgument(arguments, "speed")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	holdHours, err := optionalNonNegativeIntArgument(arguments, "holdHours")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	expeditionHours, err := optionalNonNegativeIntArgument(arguments, "expeditionHours")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	unionID, err := optionalNonNegativeIntArgument(arguments, "unionId")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	resources, err := fleetResourcesArgument(arguments, "resources")
	if err != nil {
		return domainmcp.DispatchFleetCommand{}, err
	}
	return domainmcp.DispatchFleetCommand{
		PlanetID:        planetID,
		Ships:           ships,
		Resources:       resources,
		Target:          domainmcp.Coordinates{Galaxy: targetGalaxy, System: targetSystem, Position: targetPosition},
		TargetType:      targetType,
		Mission:         mission,
		Speed:           speed,
		HoldHours:       holdHours,
		ExpeditionHours: expeditionHours,
		UnionID:         unionID,
	}, nil
}

func mcpDispatchFleetConfirmation(command domainmcp.DispatchFleetCommand) string {
	ships := stableIntMapPayload(command.Ships)
	payload := fmt.Sprintf("%d|%s|%d,%d,%d,%d|%d|%d|%d|%d|%d|%d,%d,%d",
		command.PlanetID,
		ships,
		command.Target.Galaxy,
		command.Target.System,
		command.Target.Position,
		command.TargetType,
		command.Mission,
		command.Speed,
		command.HoldHours,
		command.ExpeditionHours,
		command.UnionID,
		command.Resources.Metal,
		command.Resources.Crystal,
		command.Resources.Deuterium,
	)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("dispatch_fleet:%s", hex.EncodeToString(sum[:])[:16])
}

func mcpRecallFleetCommand(arguments map[string]any) (domainmcp.RecallFleetCommand, error) {
	fleetID, err := optionalNonNegativeIntArgument(arguments, "fleetId")
	if err != nil || fleetID <= 0 {
		return domainmcp.RecallFleetCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.RecallFleetCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.RecallFleetCommand{}, err
	}
	return domainmcp.RecallFleetCommand{FleetID: fleetID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpRecallFleetConfirmation(command domainmcp.RecallFleetCommand) string {
	payload := strconv.Itoa(command.FleetID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("recall_fleet:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpPhalanxScanCommand(arguments map[string]any) (domainmcp.PhalanxScanCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.PhalanxScanCommand{}, err
	}
	targetPlanetID, err := optionalNonNegativeIntArgument(arguments, "targetPlanetId")
	if err != nil || targetPlanetID <= 0 {
		return domainmcp.PhalanxScanCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.PhalanxScanCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.PhalanxScanCommand{}, err
	}
	return domainmcp.PhalanxScanCommand{PlanetID: planetID, TargetPlanetID: targetPlanetID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpPhalanxScanConfirmation(command domainmcp.PhalanxScanCommand) string {
	payload := fmt.Sprintf("%d:%d", command.PlanetID, command.TargetPlanetID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("scan_phalanx:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpJumpGateCommand(arguments map[string]any) (domainmcp.JumpGateCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.JumpGateCommand{}, err
	}
	sourceMoonID, err := optionalNonNegativeIntArgument(arguments, "sourceMoonId")
	if err != nil {
		return domainmcp.JumpGateCommand{}, err
	}
	targetMoonID, err := optionalNonNegativeIntArgument(arguments, "targetMoonId")
	if err != nil || targetMoonID <= 0 {
		return domainmcp.JumpGateCommand{}, domainmcp.ErrInvalidParams
	}
	ships, err := intObjectArgument(arguments, "ships")
	if err != nil || len(ships) == 0 {
		return domainmcp.JumpGateCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.JumpGateCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.JumpGateCommand{}, err
	}
	return domainmcp.JumpGateCommand{
		PlanetID:     planetID,
		SourceMoonID: sourceMoonID,
		TargetMoonID: targetMoonID,
		Ships:        ships,
		DryRun:       dryRun,
		Confirm:      confirm,
	}, nil
}

func mcpJumpGateConfirmation(command domainmcp.JumpGateCommand) string {
	payload := fmt.Sprintf("%d:%d:%d:%s", command.PlanetID, command.SourceMoonID, command.TargetMoonID, stableIntMapPayload(command.Ships))
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("jump_gate:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpCancelBuildingQueueCommand(arguments map[string]any) (domainmcp.CancelBuildingQueueCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.CancelBuildingQueueCommand{}, err
	}
	listID, err := optionalNonNegativeIntArgument(arguments, "listId")
	if err != nil || listID <= 0 {
		return domainmcp.CancelBuildingQueueCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.CancelBuildingQueueCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.CancelBuildingQueueCommand{}, err
	}
	return domainmcp.CancelBuildingQueueCommand{PlanetID: planetID, ListID: listID, DryRun: dryRun, Confirm: confirm}, nil
}

func mcpCancelBuildingQueueConfirmation(command domainmcp.CancelBuildingQueueCommand) string {
	payload := fmt.Sprintf("%d:%d", command.PlanetID, command.ListID)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("cancel_building_queue:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpCancelResearchQueueCommand(arguments map[string]any) (domainmcp.CancelResearchQueueCommand, error) {
	dryRun := true
	var err error
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.CancelResearchQueueCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.CancelResearchQueueCommand{}, err
	}
	return domainmcp.CancelResearchQueueCommand{DryRun: dryRun, Confirm: confirm}, nil
}

func mcpCancelResearchQueueConfirmation() string {
	payload := "active"
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("cancel_research_queue:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpEnqueueShipyardOrderCommand(arguments map[string]any) (domainmcp.EnqueueShipyardOrderCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	kind, err := optionalStringArgument(arguments, "kind")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "fleet"
	}
	if kind != "fleet" && kind != "defense" {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	itemID, err := optionalNonNegativeIntArgument(arguments, "itemId")
	if err != nil || itemID <= 0 {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	amount, err := optionalNonNegativeIntArgument(arguments, "amount")
	if err != nil || amount <= 0 {
		return domainmcp.EnqueueShipyardOrderCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.EnqueueShipyardOrderCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.EnqueueShipyardOrderCommand{}, err
	}
	return domainmcp.EnqueueShipyardOrderCommand{
		PlanetID: planetID,
		Kind:     kind,
		ItemID:   itemID,
		Amount:   amount,
		DryRun:   dryRun,
		Confirm:  confirm,
	}, nil
}

func mcpEnqueueShipyardOrderConfirmation(command domainmcp.EnqueueShipyardOrderCommand) string {
	payload := fmt.Sprintf("%d:%s:%d:%d", command.PlanetID, command.Kind, command.ItemID, command.Amount)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("enqueue_shipyard_order:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpUpdateResourceProductionCommand(arguments map[string]any) (domainmcp.UpdateResourceProductionCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	production, err := intObjectArgument(arguments, "production")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	if len(production) == 0 {
		return domainmcp.UpdateResourceProductionCommand{}, domainmcp.ErrInvalidParams
	}
	for _, percent := range production {
		if percent > 100 {
			return domainmcp.UpdateResourceProductionCommand{}, domainmcp.ErrInvalidParams
		}
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.UpdateResourceProductionCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.UpdateResourceProductionCommand{}, err
	}
	return domainmcp.UpdateResourceProductionCommand{
		PlanetID:   planetID,
		Production: production,
		DryRun:     dryRun,
		Confirm:    confirm,
	}, nil
}

func mcpUpdateResourceProductionConfirmation(command domainmcp.UpdateResourceProductionCommand) string {
	payload := fmt.Sprintf("%d:%s", command.PlanetID, stableIntMapPayload(command.Production))
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("update_resource_production:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpRecruitOfficerCommand(arguments map[string]any) (domainmcp.RecruitOfficerCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	officerID, err := optionalNonNegativeIntArgument(arguments, "officerId")
	if err != nil || officerID < 1 || officerID > 5 {
		return domainmcp.RecruitOfficerCommand{}, domainmcp.ErrInvalidParams
	}
	days, err := optionalNonNegativeIntArgument(arguments, "days")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	if days == 0 {
		days = 7
	}
	if days != 7 && days != 90 {
		return domainmcp.RecruitOfficerCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.RecruitOfficerCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.RecruitOfficerCommand{}, err
	}
	return domainmcp.RecruitOfficerCommand{
		PlanetID:  planetID,
		OfficerID: officerID,
		Days:      days,
		DryRun:    dryRun,
		Confirm:   confirm,
	}, nil
}

func mcpRecruitOfficerConfirmation(command domainmcp.RecruitOfficerCommand) string {
	payload := fmt.Sprintf("%d:%d:%d", command.PlanetID, command.OfficerID, command.Days)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("recruit_officer:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpSearchCommand(arguments map[string]any) (domainmcp.SearchCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.SearchCommand{}, err
	}
	searchType, err := optionalStringArgument(arguments, "type")
	if err != nil {
		return domainmcp.SearchCommand{}, err
	}
	searchType = strings.TrimSpace(searchType)
	if searchType == "" {
		searchType = "playername"
	}
	switch searchType {
	case "playername", "planetname", "allytag", "allyname":
	default:
		return domainmcp.SearchCommand{}, domainmcp.ErrInvalidParams
	}
	text, err := optionalStringArgument(arguments, "text")
	if err != nil {
		return domainmcp.SearchCommand{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return domainmcp.SearchCommand{}, domainmcp.ErrInvalidParams
	}
	return domainmcp.SearchCommand{PlanetID: planetID, Type: searchType, Text: text}, nil
}

func mcpGalaxySystemCommand(arguments map[string]any) (domainmcp.GalaxySystemCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.GalaxySystemCommand{}, err
	}
	galaxy, err := optionalNonNegativeIntArgument(arguments, "galaxy")
	if err != nil {
		return domainmcp.GalaxySystemCommand{}, err
	}
	system, err := optionalNonNegativeIntArgument(arguments, "system")
	if err != nil {
		return domainmcp.GalaxySystemCommand{}, err
	}
	return domainmcp.GalaxySystemCommand{PlanetID: planetID, Galaxy: galaxy, System: system}, nil
}

func mcpStatisticsCommand(arguments map[string]any) (domainmcp.StatisticsCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.StatisticsCommand{}, err
	}
	who, err := optionalStringArgument(arguments, "who")
	if err != nil {
		return domainmcp.StatisticsCommand{}, err
	}
	who = strings.TrimSpace(who)
	if who == "" {
		who = "player"
	}
	switch who {
	case "player", "ally":
	default:
		return domainmcp.StatisticsCommand{}, domainmcp.ErrInvalidParams
	}
	statType, err := optionalStringArgument(arguments, "type")
	if err != nil {
		return domainmcp.StatisticsCommand{}, err
	}
	statType = strings.TrimSpace(statType)
	if statType == "" {
		statType = "ressources"
	}
	if statType == "resources" {
		statType = "ressources"
	}
	switch statType {
	case "ressources", "fleet", "research":
	default:
		return domainmcp.StatisticsCommand{}, domainmcp.ErrInvalidParams
	}
	start, err := optionalNonNegativeIntArgument(arguments, "start")
	if err != nil {
		return domainmcp.StatisticsCommand{}, err
	}
	return domainmcp.StatisticsCommand{PlanetID: planetID, Who: who, Type: statType, Start: start}, nil
}

func mcpAllianceStatusCommand(arguments map[string]any) (domainmcp.AllianceStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	view, err := optionalStringArgument(arguments, "view")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	searchText, err := optionalStringArgument(arguments, "searchText")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	textKind, err := optionalNonNegativeIntArgument(arguments, "textKind")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	allianceID, err := optionalNonNegativeIntArgument(arguments, "allianceId")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	applicationID, err := optionalNonNegativeIntArgument(arguments, "applicationId")
	if err != nil {
		return domainmcp.AllianceStatusCommand{}, err
	}
	return domainmcp.AllianceStatusCommand{
		PlanetID:      planetID,
		View:          view,
		SearchText:    searchText,
		TextKind:      textKind,
		AllianceID:    allianceID,
		ApplicationID: applicationID,
	}, nil
}

func mcpBuddyStatusCommand(arguments map[string]any) (domainmcp.BuddyStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.BuddyStatusCommand{}, err
	}
	action, err := optionalNonNegativeIntArgument(arguments, "action")
	if err != nil {
		return domainmcp.BuddyStatusCommand{}, err
	}
	switch action {
	case 0, 5, 6, 7:
	default:
		return domainmcp.BuddyStatusCommand{}, domainmcp.ErrInvalidParams
	}
	buddyID, err := optionalNonNegativeIntArgument(arguments, "buddyId")
	if err != nil {
		return domainmcp.BuddyStatusCommand{}, err
	}
	return domainmcp.BuddyStatusCommand{PlanetID: planetID, Action: action, BuddyID: buddyID}, nil
}

func mcpBuddyMutationCommand(arguments map[string]any) (domainmcp.BuddyMutationCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.BuddyMutationCommand{}, err
	}
	action, err := optionalStringArgument(arguments, "action")
	if err != nil {
		return domainmcp.BuddyMutationCommand{}, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	switch action {
	case "add", "accept", "decline", "withdraw", "delete":
	default:
		return domainmcp.BuddyMutationCommand{}, domainmcp.ErrInvalidParams
	}
	buddyID, err := optionalNonNegativeIntArgument(arguments, "buddyId")
	if err != nil || buddyID <= 0 {
		return domainmcp.BuddyMutationCommand{}, domainmcp.ErrInvalidParams
	}
	text, err := optionalStringArgument(arguments, "text")
	if err != nil {
		return domainmcp.BuddyMutationCommand{}, err
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.BuddyMutationCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.BuddyMutationCommand{}, err
	}
	return domainmcp.BuddyMutationCommand{
		PlanetID: planetID,
		Action:   action,
		BuddyID:  buddyID,
		Text:     text,
		DryRun:   dryRun,
		Confirm:  confirm,
	}, nil
}

func mcpBuddyMutationConfirmation(command domainmcp.BuddyMutationCommand) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s\n%d\n%s", command.PlanetID, strings.ToLower(strings.TrimSpace(command.Action)), command.BuddyID, command.Text)))
	return fmt.Sprintf("mutate_buddy:%d:%s:%d:%s", command.PlanetID, strings.ToLower(strings.TrimSpace(command.Action)), command.BuddyID, hex.EncodeToString(sum[:])[:12])
}

func mcpPrangerCommand(arguments map[string]any) (domainmcp.PrangerCommand, error) {
	universe, err := optionalNonNegativeIntArgument(arguments, "universe")
	if err != nil {
		return domainmcp.PrangerCommand{}, err
	}
	from, err := optionalNonNegativeIntArgument(arguments, "from")
	if err != nil {
		return domainmcp.PrangerCommand{}, err
	}
	return domainmcp.PrangerCommand{Universe: universe, From: from}, nil
}

func mcpNotesStatusCommand(arguments map[string]any) (domainmcp.NotesStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.NotesStatusCommand{}, err
	}
	action, err := optionalNonNegativeIntArgument(arguments, "action")
	if err != nil {
		return domainmcp.NotesStatusCommand{}, err
	}
	switch action {
	case 0, 1, 2:
	default:
		return domainmcp.NotesStatusCommand{}, domainmcp.ErrInvalidParams
	}
	noteID, err := optionalNonNegativeIntArgument(arguments, "noteId")
	if err != nil {
		return domainmcp.NotesStatusCommand{}, err
	}
	return domainmcp.NotesStatusCommand{PlanetID: planetID, Action: action, NoteID: noteID}, nil
}

func mcpNoteMutationCommand(arguments map[string]any, requireNoteID bool, requireNoteIDs bool) (domainmcp.NoteMutationCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	noteID, err := optionalNonNegativeIntArgument(arguments, "noteId")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	if requireNoteID && noteID <= 0 {
		return domainmcp.NoteMutationCommand{}, domainmcp.ErrInvalidParams
	}
	noteIDs, err := positiveIntSliceArgument(arguments, "noteIds")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	if requireNoteIDs && len(noteIDs) == 0 {
		return domainmcp.NoteMutationCommand{}, domainmcp.ErrInvalidParams
	}
	subject, err := optionalStringArgument(arguments, "subject")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	text, err := optionalStringArgument(arguments, "text")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	priority, err := optionalNonNegativeIntArgument(arguments, "priority")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	if priority > 2 {
		return domainmcp.NoteMutationCommand{}, domainmcp.ErrInvalidParams
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.NoteMutationCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.NoteMutationCommand{}, err
	}
	return domainmcp.NoteMutationCommand{
		PlanetID: planetID,
		NoteID:   noteID,
		Subject:  subject,
		Text:     text,
		Priority: priority,
		NoteIDs:  noteIDs,
		DryRun:   dryRun,
		Confirm:  confirm,
	}, nil
}

func mcpCreateNoteConfirmation(command domainmcp.NoteMutationCommand) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s\n%s\n%d", command.PlanetID, command.Subject, command.Text, command.Priority)))
	return fmt.Sprintf("create_note:%d:%s", command.PlanetID, hex.EncodeToString(sum[:])[:12])
}

func mcpUpdateNoteConfirmation(command domainmcp.NoteMutationCommand) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%d\n%s\n%s\n%d", command.PlanetID, command.NoteID, command.Subject, command.Text, command.Priority)))
	return fmt.Sprintf("update_note:%d:%d:%s", command.PlanetID, command.NoteID, hex.EncodeToString(sum[:])[:12])
}

func mcpDeleteNotesConfirmation(command domainmcp.NoteMutationCommand) string {
	ids := make([]string, 0, len(command.NoteIDs))
	for _, id := range command.NoteIDs {
		ids = append(ids, strconv.Itoa(id))
	}
	payload := strings.Join(ids, ",")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\n%s", command.PlanetID, payload)))
	return fmt.Sprintf("delete_notes:%d:%s:%s", command.PlanetID, payload, hex.EncodeToString(sum[:])[:12])
}

func mcpOptionsStatusCommand(arguments map[string]any) (domainmcp.OptionsStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.OptionsStatusCommand{}, err
	}
	return domainmcp.OptionsStatusCommand{PlanetID: planetID}, nil
}

func mcpMerchantStatusCommand(arguments map[string]any) (domainmcp.MerchantStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.MerchantStatusCommand{}, err
	}
	return domainmcp.MerchantStatusCommand{PlanetID: planetID}, nil
}

func mcpMerchantMutationCommand(arguments map[string]any) (domainmcp.MerchantMutationCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.MerchantMutationCommand{}, err
	}
	action, err := optionalStringArgument(arguments, "action")
	if err != nil {
		return domainmcp.MerchantMutationCommand{}, err
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "call" && action != "trade" {
		return domainmcp.MerchantMutationCommand{}, domainmcp.ErrInvalidParams
	}
	offerID, err := optionalNonNegativeIntArgument(arguments, "offerId")
	if err != nil {
		return domainmcp.MerchantMutationCommand{}, err
	}
	values, err := merchantTradeValuesArgument(arguments, "values")
	if err != nil {
		return domainmcp.MerchantMutationCommand{}, err
	}
	switch action {
	case "call":
		if offerID < 1 || offerID > 3 {
			return domainmcp.MerchantMutationCommand{}, domainmcp.ErrInvalidParams
		}
	case "trade":
		if values.Metal+values.Crystal+values.Deuterium <= 0 {
			return domainmcp.MerchantMutationCommand{}, domainmcp.ErrInvalidParams
		}
	}
	dryRun := true
	if arguments != nil && arguments["dryRun"] != nil {
		dryRun, err = optionalBoolArgument(arguments, "dryRun")
		if err != nil {
			return domainmcp.MerchantMutationCommand{}, err
		}
	}
	confirm, err := optionalStringArgument(arguments, "confirm")
	if err != nil {
		return domainmcp.MerchantMutationCommand{}, err
	}
	return domainmcp.MerchantMutationCommand{
		PlanetID: planetID,
		Action:   action,
		OfferID:  offerID,
		Values:   values,
		DryRun:   dryRun,
		Confirm:  confirm,
	}, nil
}

func merchantTradeValuesArgument(arguments map[string]any, name string) (domainmcp.MerchantTradeValues, error) {
	if arguments == nil || arguments[name] == nil {
		return domainmcp.MerchantTradeValues{}, nil
	}
	object, ok := arguments[name].(map[string]any)
	if !ok {
		return domainmcp.MerchantTradeValues{}, fmt.Errorf("%w: %s must be an object", domainmcp.ErrInvalidParams, name)
	}
	metal, err := optionalObjectNonNegativeInt(object, "metal")
	if err != nil {
		return domainmcp.MerchantTradeValues{}, err
	}
	crystal, err := optionalObjectNonNegativeInt(object, "crystal")
	if err != nil {
		return domainmcp.MerchantTradeValues{}, err
	}
	deuterium, err := optionalObjectNonNegativeInt(object, "deuterium")
	if err != nil {
		return domainmcp.MerchantTradeValues{}, err
	}
	return domainmcp.MerchantTradeValues{Metal: metal, Crystal: crystal, Deuterium: deuterium}, nil
}

func mcpMerchantMutationConfirmation(command domainmcp.MerchantMutationCommand) string {
	payload := fmt.Sprintf("%d:%s:%d:%d:%d:%d", command.PlanetID, command.Action, command.OfferID, command.Values.Metal, command.Values.Crystal, command.Values.Deuterium)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("mutate_merchant:%s:%s", payload, hex.EncodeToString(sum[:])[:12])
}

func mcpJumpGateStatusCommand(arguments map[string]any) (domainmcp.JumpGateStatusCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.JumpGateStatusCommand{}, err
	}
	return domainmcp.JumpGateStatusCommand{PlanetID: planetID}, nil
}

func mcpEmpireCommand(arguments map[string]any) (domainmcp.EmpireCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.EmpireCommand{}, err
	}
	planetType, err := optionalNonNegativeIntArgument(arguments, "planetType")
	if err != nil {
		return domainmcp.EmpireCommand{}, err
	}
	switch planetType {
	case 0, 1, 3:
	default:
		return domainmcp.EmpireCommand{}, domainmcp.ErrInvalidParams
	}
	return domainmcp.EmpireCommand{PlanetID: planetID, PlanetType: planetType}, nil
}

func mcpTechnologyCommand(arguments map[string]any) (domainmcp.TechnologyCommand, error) {
	planetID, err := optionalNonNegativeIntArgument(arguments, "planetId")
	if err != nil {
		return domainmcp.TechnologyCommand{}, err
	}
	detailsID, err := optionalNonNegativeIntArgument(arguments, "detailsId")
	if err != nil {
		return domainmcp.TechnologyCommand{}, err
	}
	infoID, err := optionalNonNegativeIntArgument(arguments, "infoId")
	if err != nil {
		return domainmcp.TechnologyCommand{}, err
	}
	return domainmcp.TechnologyCommand{PlanetID: planetID, DetailsID: detailsID, InfoID: infoID}, nil
}

func mcpMessageQuery(arguments map[string]any) (domainmcp.MessageQuery, error) {
	limit, err := optionalNonNegativeIntArgument(arguments, "limit")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 50 {
		limit = 50
	}
	messageType, err := optionalNonNegativeIntArgument(arguments, "messageType")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	includeText, err := optionalBoolArgument(arguments, "includeText")
	if err != nil {
		return domainmcp.MessageQuery{}, err
	}
	return domainmcp.MessageQuery{
		Limit:          limit,
		MessageType:    messageType,
		HasMessageType: arguments != nil && arguments["messageType"] != nil,
		IncludeText:    includeText,
	}, nil
}

func optionalStringArgument(arguments map[string]any, name string) (string, error) {
	if arguments == nil {
		return "", nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a string", domainmcp.ErrInvalidParams, name)
	}
	return value, nil
}

func positiveIntSliceArgument(arguments map[string]any, name string) ([]int, error) {
	if arguments == nil {
		return nil, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an array", domainmcp.ErrInvalidParams, name)
	}
	seen := map[int]struct{}{}
	result := make([]int, 0, len(values))
	for _, value := range values {
		id, err := nonNegativeIntValue(value, name)
		if err != nil {
			return nil, err
		}
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Ints(result)
	return result, nil
}

func intObjectArgument(arguments map[string]any, name string) (map[int]int, error) {
	if arguments == nil {
		return nil, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return nil, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %s must be an object", domainmcp.ErrInvalidParams, name)
	}
	values := map[int]int{}
	for key, rawValue := range object {
		id, err := strconv.Atoi(strings.TrimSpace(key))
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%w: %s contains an invalid id", domainmcp.ErrInvalidParams, name)
		}
		count, err := nonNegativeIntValue(rawValue, name+"."+key)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			values[id] = count
		}
	}
	return values, nil
}

func fleetResourcesArgument(arguments map[string]any, name string) (domainmcp.FleetResources, error) {
	if arguments == nil || arguments[name] == nil {
		return domainmcp.FleetResources{}, nil
	}
	object, ok := arguments[name].(map[string]any)
	if !ok {
		return domainmcp.FleetResources{}, fmt.Errorf("%w: %s must be an object", domainmcp.ErrInvalidParams, name)
	}
	metal, err := optionalObjectNonNegativeInt(object, "metal")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	crystal, err := optionalObjectNonNegativeInt(object, "crystal")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	deuterium, err := optionalObjectNonNegativeInt(object, "deuterium")
	if err != nil {
		return domainmcp.FleetResources{}, err
	}
	return domainmcp.FleetResources{Metal: metal, Crystal: crystal, Deuterium: deuterium}, nil
}

func optionalObjectNonNegativeInt(object map[string]any, name string) (int, error) {
	raw, ok := object[name]
	if !ok || raw == nil {
		return 0, nil
	}
	return nonNegativeIntValue(raw, name)
}

func stableIntMapPayload(values map[int]int) string {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", key, values[key]))
	}
	return strings.Join(parts, ",")
}

func optionalBoolArgument(arguments map[string]any, name string) (bool, error) {
	if arguments == nil {
		return false, nil
	}
	raw, ok := arguments[name]
	if !ok || raw == nil {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%w: %s must be a boolean", domainmcp.ErrInvalidParams, name)
	}
	return value, nil
}

func (s Service) verify(ctx context.Context, token string) (domainmcp.Access, error) {
	token = strings.TrimSpace(token)
	if token == "" || s.verifier == nil {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	access, err := s.verifier.VerifyMCPToken(ctx, token)
	if err != nil {
		return domainmcp.Access{}, err
	}
	if !access.Authenticated {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	return access, nil
}

func callMCPAccess(access domainmcp.Access) domainmcp.ToolCallResult {
	structured := map[string]any{
		"authenticated": access.Authenticated,
		"playerId":      access.PlayerID,
		"scopes":        access.Scopes,
	}
	text, _ := json.Marshal(structured)
	return domainmcp.ToolCallResult{
		Content: []domainmcp.Content{
			{Type: "text", Text: string(text)},
		},
		StructuredContent: structured,
		IsError:           false,
	}
}

func serverHealthTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_server_health",
		Title:       "Get Server Health",
		Description: "Return public Go migration runtime and readiness information for this OGame server.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":            map[string]any{"type": "string"},
				"service":           map[string]any{"type": "string"},
				"environment":       map[string]any{"type": "string"},
				"runtime":           map[string]any{"type": "string"},
				"goTarget":          map[string]any{"type": "string"},
				"bunTarget":         map[string]any{"type": "string"},
				"reactTarget":       map[string]any{"type": "string"},
				"staticReady":       map[string]any{"type": "boolean"},
				"legacyAssetsReady": map[string]any{"type": "boolean"},
				"legacyBaseUrl":     map[string]any{"type": "string"},
			},
			"required": []string{"status", "service", "environment", "runtime"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func accessTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_mcp_access",
		Title:       "Get MCP Access",
		Description: "Return the authenticated MCP player id and granted scopes for the current bearer token.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"authenticated": map[string]any{"type": "boolean"},
				"playerId":      map[string]any{"type": "integer"},
				"scopes": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"required": []string{"authenticated", "playerId", "scopes"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func listPlanetsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "list_planets",
		Title:       "List Planets",
		Description: "Return the authenticated player's selectable planets and moons using the legacy planet switcher ordering.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"playerId": map[string]any{"type": "integer"},
				"count":    map[string]any{"type": "integer"},
				"planets": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":       map[string]any{"type": "integer"},
							"name":     map[string]any{"type": "string"},
							"type":     map[string]any{"type": "integer"},
							"typeName": map[string]any{"type": "string"},
							"coordinates": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"galaxy":   map[string]any{"type": "integer"},
									"system":   map[string]any{"type": "integer"},
									"position": map[string]any{"type": "integer"},
								},
								"required": []string{"galaxy", "system", "position"},
							},
							"current": map[string]any{"type": "boolean"},
						},
						"required": []string{"id", "name", "type", "typeName", "coordinates", "current"},
					},
				},
			},
			"required": []string{"playerId", "count", "planets"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func accountOverviewTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_account_overview",
		Title:       "Get Account Overview",
		Description: "Return read-only account summary data for the authenticated player without triggering legacy overview mutations.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"overview": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":       map[string]any{"type": "integer"},
						"commander":      map[string]any{"type": "string"},
						"planetCount":    map[string]any{"type": "integer"},
						"unreadMessages": map[string]any{"type": "integer"},
						"score": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"raw":     map[string]any{"type": "integer"},
								"display": map[string]any{"type": "integer"},
								"rank":    map[string]any{"type": "integer"},
							},
							"required": []string{"raw", "display", "rank"},
						},
						"currentPlanet": map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "commander", "score", "currentPlanet", "planetCount", "unreadMessages"},
				},
			},
			"required": []string{"overview"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func planetResourcesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_planet_resources",
		Title:       "Get Planet Resources",
		Description: "Return read-only resource, storage, energy, and hourly production values for the current or requested owned planet.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional owned planet id. Omit or pass 0 to use the current active planet.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resources": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":          map[string]any{"type": "integer"},
						"planet":            map[string]any{"type": "object"},
						"resources":         map[string]any{"type": "object"},
						"capacity":          map[string]any{"type": "object"},
						"energy":            map[string]any{"type": "object"},
						"productionPerHour": map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "resources", "capacity", "energy", "productionPerHour"},
				},
			},
			"required": []string{"resources"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func buildingQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_building_queue",
		Title:       "Get Building Queue",
		Description: "Return read-only building queue entries for the current or requested owned planet without finishing or mutating queued work.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional owned planet id. Omit or pass 0 to use the current active planet.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"buildingQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"planet":   map[string]any{"type": "object"},
						"count":    map[string]any{"type": "integer"},
						"entries": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"listId":           map[string]any{"type": "integer"},
									"techId":           map[string]any{"type": "integer"},
									"name":             map[string]any{"type": "string"},
									"level":            map[string]any{"type": "integer"},
									"destroy":          map[string]any{"type": "boolean"},
									"start":            map[string]any{"type": "integer"},
									"end":              map[string]any{"type": "integer"},
									"remainingSeconds": map[string]any{"type": "integer"},
								},
								"required": []string{"listId", "techId", "name", "level", "destroy", "start", "end", "remainingSeconds"},
							},
						},
					},
					"required": []string{"playerId", "planet", "count", "entries"},
				},
			},
			"required": []string{"buildingQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func listMessagesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "list_messages",
		Title:       "List Messages",
		Description: "Return the authenticated player's message inbox rows without marking messages read or deleting expired rows.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"limit": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"maximum":     50,
					"description": "Optional row limit. Defaults to 25 and caps at 50.",
				},
				"messageType": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Optional legacy message type filter.",
				},
				"includeText": map[string]any{
					"type":        "boolean",
					"description": "Include message body text when true. Defaults to false.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messages": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"count":    map[string]any{"type": "integer"},
						"limit":    map[string]any{"type": "integer"},
						"messages": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
					"required": []string{"playerId", "count", "limit", "messages"},
				},
			},
			"required": []string{"messages"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func getMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_message",
		Title:       "Get Message",
		Description: "Return one owned message by id without marking it read.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned message id.",
				},
			},
			"required":             []string{"messageId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"message":  map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "message"},
				},
			},
			"required": []string{"message"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func getReportTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_report",
		Title:       "Get Report",
		Description: "Return an owned battle or espionage report body by id using legacy report access rules.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reportId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Legacy report message id.",
				},
			},
			"required":             []string{"reportId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"report": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"id":       map[string]any{"type": "integer"},
						"type":     map[string]any{"type": "integer"},
						"title":    map[string]any{"type": "string"},
						"text":     map[string]any{"type": "string"},
						"allowed":  map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "id", "type", "title", "text", "allowed"},
				},
			},
			"required": []string{"report"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func sendMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "send_message",
		Title:       "Send Message",
		Description: "Dry-run or explicitly confirm sending an in-game private message. Defaults to dry-run; confirmed execution requires the exact confirmation string returned by dry-run.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"targetPlayerId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Recipient player id.",
				},
				"subject": map[string]any{
					"type":        "string",
					"description": "Message subject. Legacy rules truncate to 40 characters.",
				},
				"text": map[string]any{
					"type":        "string",
					"description": "Message body. Legacy rules truncate to 2000 characters.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same payload.",
				},
			},
			"required":             []string{"targetPlayerId", "subject", "text"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sendMessage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"targetPlayerId":       map[string]any{"type": "integer"},
						"subject":              map[string]any{"type": "string"},
						"textChars":            map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "targetPlayerId", "subject", "textChars", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"sendMessage"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": false,
			"idempotentHint":  false,
		},
	}
}

func deleteMessagesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "delete_messages",
		Title:       "Delete Messages",
		Description: "Dry-run or explicitly confirm deleting selected owned inbox messages. Only positive messageIds are accepted; broad delete-all modes are not exposed.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageIds": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "integer", "minimum": 1},
					"description": "Owned inbox message ids to delete.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same messageIds.",
				},
			},
			"required":             []string{"messageIds"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"deleteMessages": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"messageIds":           map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"deleteCount":          map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "messageIds", "deleteCount", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"deleteMessages"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func reportMessageTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "report_message",
		Title:       "Report Message",
		Description: "Dry-run or explicitly confirm reporting one owned inbox private message. Non-PM and non-visible messages are not reported.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"messageId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned visible private-message id to report.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same messageId.",
				},
			},
			"required":             []string{"messageId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"reportMessage": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"messageId":            map[string]any{"type": "integer"},
						"reportable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "messageId", "reportable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"reportMessage"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func fleetMovementsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_fleet_movements",
		Title:       "Get Fleet Movements",
		Description: "Return read-only overview-style fleet movements for the authenticated player, including incoming, outgoing, return, hold, and ACS grouped events.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetMovements": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"now":      map[string]any{"type": "integer"},
						"count":    map[string]any{"type": "integer"},
						"events": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
					"required": []string{"playerId", "now", "count", "events"},
				},
			},
			"required": []string{"fleetMovements"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func resourceProductionOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_resource_production_options",
		Title:       "Get Resource Production Options",
		Description: "Return read-only legacy resources screen production settings, row outputs, storage, and totals for the authenticated player's current or selected planet without updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceProductionOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"planet":   map[string]any{"type": "object"},
						"factor":   map[string]any{"type": "number"},
						"natural":  map[string]any{"type": "object"},
						"rows":     map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"storage":  map[string]any{"type": "object"},
						"totals":   map[string]any{"type": "object"},
						"settings": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "factor", "natural", "rows", "storage", "totals", "settings"},
				},
			},
			"required": []string{"resourceProductionOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func fleetOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_fleet_options",
		Title:       "Get Fleet Options",
		Description: "Return read-only legacy fleet screen state, including slots, selectable ships, templates, active missions, and optional dispatch draft for the authenticated player's current or selected planet.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planet":          map[string]any{"type": "object"},
						"commanderActive": map[string]any{"type": "boolean"},
						"slots":           map[string]any{"type": "object"},
						"expeditions":     map[string]any{"type": "object"},
						"expeditionLevel": map[string]any{"type": "integer"},
						"speedFactor":     map[string]any{"type": "integer"},
						"missions":        map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"ships":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"templateLimit":   map[string]any{"type": "integer"},
						"templates":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"dispatchDraft":   map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "commanderActive", "slots", "expeditions", "expeditionLevel", "speedFactor", "missions", "ships", "templateLimit", "templates"},
				},
			},
			"required": []string{"fleetOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func officerStatusTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_officer_status",
		Title:       "Get Officer Status",
		Description: "Return read-only commander/officer status, remaining days, costs, and Dark Matter balances for the authenticated player.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"officerStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":       map[string]any{"type": "integer"},
						"planetId":       map[string]any{"type": "integer"},
						"paidDarkMatter": map[string]any{"type": "integer"},
						"freeDarkMatter": map[string]any{"type": "integer"},
						"officers": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "object"},
						},
					},
					"required": []string{"playerId", "planetId", "paidDarkMatter", "freeDarkMatter", "officers"},
				},
			},
			"required": []string{"officerStatus"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func searchGameTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "search_game",
		Title:       "Search Game",
		Description: "Search players, planets, alliance tags, or alliance names using the legacy authenticated search behavior.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Search type. Defaults to playername.",
					"enum":        []string{"playername", "planetname", "allytag", "allyname"},
				},
				"text": map[string]any{
					"type":        "string",
					"description": "Search text. Legacy search returns a too-short message for one-character input.",
					"minLength":   1,
				},
			},
			"required":             []string{"text"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"search": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":  map[string]any{"type": "integer"},
						"planetId":  map[string]any{"type": "integer"},
						"type":      map[string]any{"type": "string"},
						"text":      map[string]any{"type": "string"},
						"message":   map[string]any{"type": "string"},
						"players":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"alliances": map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planetId", "type", "text", "players", "alliances"},
				},
			},
			"required": []string{"search"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func galaxySystemTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_galaxy_system",
		Title:       "Get Galaxy System",
		Description: "Return a read-only galaxy system view. Unlike the legacy page, MCP read mode does not charge remote-system deuterium.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"galaxy": map[string]any{
					"type":        "integer",
					"description": "Galaxy number. Omit or pass 0 to use the active planet galaxy.",
					"minimum":     0,
				},
				"system": map[string]any{
					"type":        "integer",
					"description": "System number. Omit or pass 0 to use the active planet system.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"galaxySystem": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":            map[string]any{"type": "integer"},
						"planetId":            map[string]any{"type": "integer"},
						"coordinates":         map[string]any{"type": "object"},
						"bounds":              map[string]any{"type": "object"},
						"populated":           map[string]any{"type": "integer"},
						"slots":               map[string]any{"type": "object"},
						"extra":               map[string]any{"type": "object"},
						"notEnoughDeuterium":  map[string]any{"type": "boolean"},
						"remoteSystemCostDue": map[string]any{"type": "boolean"},
						"rows":                map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planetId", "coordinates", "bounds", "populated", "slots", "extra", "notEnoughDeuterium", "remoteSystemCostDue", "rows"},
				},
			},
			"required": []string{"galaxySystem"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func statisticsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_statistics",
		Title:       "Get Statistics",
		Description: "Return read-only legacy statistics rankings for players or alliances by resources, fleet, or research score.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"who": map[string]any{
					"type":        "string",
					"description": "Ranking target. Defaults to player.",
					"enum":        []string{"player", "ally"},
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Ranking score type. Defaults to legacy ressources.",
					"enum":        []string{"ressources", "resources", "fleet", "research"},
				},
				"start": map[string]any{
					"type":        "integer",
					"description": "One-based rank page start. Omit or pass 0 to use the authenticated player's own rank page.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"statistics": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":         map[string]any{"type": "integer"},
						"planetId":         map[string]any{"type": "integer"},
						"viewerAllianceId": map[string]any{"type": "integer"},
						"who":              map[string]any{"type": "string"},
						"type":             map[string]any{"type": "string"},
						"start":            map[string]any{"type": "integer"},
						"total":            map[string]any{"type": "integer"},
						"generatedAt":      map[string]any{"type": "integer"},
						"rows":             map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planetId", "viewerAllianceId", "who", "type", "start", "total", "generatedAt", "rows"},
				},
			},
			"required": []string{"statistics"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func allianceStatusTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_alliance_status",
		Title:       "Get Alliance Status",
		Description: "Return read-only legacy alliance screen state for the authenticated player, including no-alliance, search, info, members, applications, ranks, management, and circular views.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"view": map[string]any{
					"type":        "string",
					"description": "Legacy alliance view.",
					"enum":        []string{"", "home", "no_alliance", "create", "search", "info", "apply", "applications", "members", "management", "ranks", "circular", "rename_tag", "rename_name"},
				},
				"searchText": map[string]any{
					"type":        "string",
					"description": "Search text for the search view.",
				},
				"textKind": map[string]any{
					"type":        "integer",
					"description": "Legacy alliance text section selector for management.",
					"minimum":     0,
				},
				"allianceId": map[string]any{
					"type":        "integer",
					"description": "Target alliance id for info/apply views.",
					"minimum":     0,
				},
				"applicationId": map[string]any{
					"type":        "integer",
					"description": "Target application id for applications view.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"allianceStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":       map[string]any{"type": "integer"},
						"planet":         map[string]any{"type": "object"},
						"view":           map[string]any{"type": "string"},
						"viewer":         map[string]any{"type": "object"},
						"own":            map[string]any{"type": "object"},
						"target":         map[string]any{"type": "object"},
						"pending":        map[string]any{"type": "object"},
						"searchText":     map[string]any{"type": "string"},
						"textKind":       map[string]any{"type": "integer"},
						"searchResults":  map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"applications":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"selectedApp":    map[string]any{"type": "object"},
						"members":        map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"ranks":          map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"circularResult": map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "view", "viewer", "searchText", "textKind", "searchResults", "applications", "members", "ranks"},
				},
			},
			"required": []string{"allianceStatus"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func buddyStatusTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_buddy_status",
		Title:       "Get Buddy Status",
		Description: "Return read-only legacy buddy screen state for the authenticated player, including accepted, incoming, outgoing, and request-target views.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
				"action": map[string]any{
					"type":        "integer",
					"description": "Legacy buddy view action: 0 home, 5 incoming, 6 outgoing, 7 request target.",
					"enum":        []int{0, 5, 6, 7},
				},
				"buddyId": map[string]any{
					"type":        "integer",
					"description": "Target player id for action 7.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"buddyStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":  map[string]any{"type": "integer"},
						"planet":    map[string]any{"type": "object"},
						"commander": map[string]any{"type": "string"},
						"action":    map[string]any{"type": "integer"},
						"rows":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"target":    map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "commander", "action", "rows"},
				},
			},
			"required": []string{"buddyStatus"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func mutateBuddyTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "mutate_buddy",
		Title:       "Mutate Buddy",
		Description: "Send, accept, decline, withdraw, or delete a legacy buddy relationship. Defaults to dry-run and requires confirmation for execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Buddy mutation action.",
					"enum":        []string{"add", "accept", "decline", "withdraw", "delete"},
				},
				"buddyId": map[string]any{
					"type":        "integer",
					"description": "Target player id for add, or legacy buddy relation id for other actions.",
					"minimum":     1,
				},
				"text":    map[string]any{"type": "string", "description": "Optional request text for add."},
				"dryRun":  map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
				"confirm": map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
			},
			"required":             []string{"action", "buddyId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"buddyMutation": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"action":               map[string]any{"type": "string"},
						"legacyAction":         map[string]any{"type": "integer"},
						"buddyId":              map[string]any{"type": "integer"},
						"textChars":            map[string]any{"type": "integer"},
						"status":               map[string]any{"type": "object"},
						"issue":                map[string]any{"type": "object"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "planetId", "action", "legacyAction", "buddyId", "textChars", "status", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"buddyMutation"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func prangerTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_pranger",
		Title:       "Get Pranger",
		Description: "Return read-only legacy pranger ban-list rows with pagination metadata.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"universe": map[string]any{"type": "integer", "minimum": 0, "description": "Optional universe number for display metadata."},
				"from":     map[string]any{"type": "integer", "minimum": 0, "description": "Optional zero-based row offset."},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pranger": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":    map[string]any{"type": "integer"},
						"universe":    map[string]any{"type": "integer"},
						"from":        map[string]any{"type": "integer"},
						"limit":       map[string]any{"type": "integer"},
						"hasPrevious": map[string]any{"type": "boolean"},
						"previous":    map[string]any{"type": "integer"},
						"hasNext":     map[string]any{"type": "boolean"},
						"next":        map[string]any{"type": "integer"},
						"entries":     map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "universe", "from", "limit", "hasPrevious", "previous", "hasNext", "next", "entries"},
				},
			},
			"required": []string{"pranger"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func notesTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_notes",
		Title:       "Get Notes",
		Description: "Return read-only legacy notes screen state for the authenticated player, including list, create form, and edit-target views.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
				"action": map[string]any{
					"type":        "integer",
					"description": "Legacy notes action: 0 list, 1 create, 2 edit.",
					"enum":        []int{0, 1, 2},
				},
				"noteId": map[string]any{
					"type":        "integer",
					"description": "Owned note id for edit action.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"notes": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":  map[string]any{"type": "integer"},
						"planet":    map[string]any{"type": "object"},
						"commander": map[string]any{"type": "string"},
						"action":    map[string]any{"type": "string"},
						"rows":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"editNote":  map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "commander", "action", "rows"},
				},
			},
			"required": []string{"notes"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func createNoteTool() domainmcp.Tool {
	return noteMutationTool("create_note", "Create Note", "Create a legacy note for the authenticated player. Defaults to dry-run and requires the returned confirmation for execution.", map[string]any{
		"planetId": notePlanetIDSchema(),
		"subject":  map[string]any{"type": "string", "description": "Note subject. Legacy normalization defaults and truncates it."},
		"text":     map[string]any{"type": "string", "description": "Note body. Legacy normalization defaults and truncates it."},
		"priority": map[string]any{"type": "integer", "minimum": 0, "maximum": 2, "description": "Legacy priority: 0 normal, 1 important, 2 urgent."},
		"dryRun":   map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
		"confirm":  map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
	}, []string{})
}

func updateNoteTool() domainmcp.Tool {
	return noteMutationTool("update_note", "Update Note", "Update one owned legacy note. Defaults to dry-run and requires the returned confirmation for execution.", map[string]any{
		"planetId": notePlanetIDSchema(),
		"noteId":   map[string]any{"type": "integer", "minimum": 1, "description": "Owned note id to update."},
		"subject":  map[string]any{"type": "string", "description": "Replacement subject. Legacy normalization defaults and truncates it."},
		"text":     map[string]any{"type": "string", "description": "Replacement body. Legacy normalization defaults and truncates it."},
		"priority": map[string]any{"type": "integer", "minimum": 0, "maximum": 2, "description": "Legacy priority: 0 normal, 1 important, 2 urgent."},
		"dryRun":   map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
		"confirm":  map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
	}, []string{"noteId"})
}

func deleteNotesTool() domainmcp.Tool {
	return noteMutationTool("delete_notes", "Delete Notes", "Delete owned legacy notes by id. Defaults to dry-run and requires the returned confirmation when matching rows exist.", map[string]any{
		"planetId": notePlanetIDSchema(),
		"noteIds": map[string]any{
			"type":        "array",
			"description": "Owned note ids to delete. Duplicates are ignored.",
			"items":       map[string]any{"type": "integer", "minimum": 1},
		},
		"dryRun":  map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
		"confirm": map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
	}, []string{"noteIds"})
}

func notePlanetIDSchema() map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
		"minimum":     0,
	}
}

func noteMutationTool(name string, title string, description string, properties map[string]any, required []string) domainmcp.Tool {
	inputSchema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		inputSchema["required"] = required
	}
	return domainmcp.Tool{
		Name:        name,
		Title:       title,
		Description: description,
		InputSchema: inputSchema,
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				toolStructuredName(name): map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"noteId":               map[string]any{"type": "integer"},
						"noteIds":              map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
						"deleteCount":          map[string]any{"type": "integer"},
						"notes":                map[string]any{"type": "object"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "planetId", "notes", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{toolStructuredName(name)},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func toolStructuredName(toolName string) string {
	switch toolName {
	case "create_note":
		return "createNote"
	case "update_note":
		return "updateNote"
	case "delete_notes":
		return "deleteNotes"
	default:
		return toolName
	}
}

func optionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_options",
		Title:       "Get Options",
		Description: "Return read-only legacy options screen state for the authenticated player without password hashes or feed secrets.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"options": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":  map[string]any{"type": "integer"},
						"planet":    map[string]any{"type": "object"},
						"commander": map[string]any{"type": "string"},
						"user":      map[string]any{"type": "object"},
						"universe":  map[string]any{"type": "object"},
						"settings":  map[string]any{"type": "object"},
						"account":   map[string]any{"type": "object"},
						"flags":     map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "commander", "user", "universe", "settings", "account", "flags"},
				},
			},
			"required": []string{"options"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func maintenanceTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_maintenance",
		Title:       "Get Maintenance",
		Description: "Return read-only universe maintenance state for the authenticated player.",
		InputSchema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"maintenance": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"frozen":   map[string]any{"type": "boolean"},
						"language": map[string]any{"type": "string"},
						"boardUrl": map[string]any{"type": "string"},
					},
					"required": []string{"playerId", "frozen", "language", "boardUrl"},
				},
			},
			"required": []string{"maintenance"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func merchantStatusTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_merchant_status",
		Title:       "Get Merchant Status",
		Description: "Return read-only legacy merchant screen state for the authenticated player, including active offer, rates, dark matter, and trade resource rows.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"merchantStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":      map[string]any{"type": "integer"},
						"planet":        map[string]any{"type": "object"},
						"commander":     map[string]any{"type": "string"},
						"user":          map[string]any{"type": "object"},
						"activeOfferId": map[string]any{"type": "integer"},
						"rates":         map[string]any{"type": "object"},
						"rows":          map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "commander", "user", "activeOfferId", "rates", "rows"},
				},
			},
			"required": []string{"merchantStatus"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func mutateMerchantTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "mutate_merchant",
		Title:       "Mutate Merchant",
		Description: "Call a legacy merchant offer or execute an active merchant trade. Defaults to dry-run and requires confirmation for execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
					"minimum":     0,
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Merchant mutation action.",
					"enum":        []string{"call", "trade"},
				},
				"offerId": map[string]any{
					"type":        "integer",
					"description": "Resource offered by a merchant call: 1 metal, 2 crystal, 3 deuterium.",
					"enum":        []int{1, 2, 3},
				},
				"values": map[string]any{
					"type":        "object",
					"description": "Requested resources for trade action.",
					"properties": map[string]any{
						"metal":     map[string]any{"type": "integer", "minimum": 0},
						"crystal":   map[string]any{"type": "integer", "minimum": 0},
						"deuterium": map[string]any{"type": "integer", "minimum": 0},
					},
					"additionalProperties": false,
				},
				"dryRun":  map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
				"confirm": map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
			},
			"required":             []string{"action"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"merchantMutation": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"action":               map[string]any{"type": "string"},
						"offerId":              map[string]any{"type": "integer"},
						"values":               map[string]any{"type": "object"},
						"status":               map[string]any{"type": "object"},
						"issue":                map[string]any{"type": "object"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
					},
					"required": []string{"playerId", "planetId", "action", "values", "status", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"merchantMutation"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": false,
			"idempotentHint":  false,
		},
	}
}

func jumpGateStatusTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_jump_gate_status",
		Title:       "Get Jump Gate Status",
		Description: "Return read-only legacy jump gate screen state for the authenticated player, including source moon, targets, ships, and current issue.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet or moon id. Omit or pass 0 to use the active context.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"jumpGateStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":  map[string]any{"type": "integer"},
						"planet":    map[string]any{"type": "object"},
						"commander": map[string]any{"type": "string"},
						"source":    map[string]any{"type": "object"},
						"targets":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"ships":     map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"issue":     map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planet", "commander", "source", "targets", "ships"},
				},
			},
			"required": []string{"jumpGateStatus"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func empireOverviewTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_empire_overview",
		Title:       "Get Empire Overview",
		Description: "Return a read-only empire overview across the authenticated player's planets or moons without finishing queues or updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"planetType": map[string]any{
					"type":        "integer",
					"description": "0 or 1 for planets, 3 for moons when enabled.",
					"enum":        []int{0, 1, 3},
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"empire": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planetId":        map[string]any{"type": "integer"},
						"commanderActive": map[string]any{"type": "boolean"},
						"planetType":      map[string]any{"type": "integer"},
						"moonEnabled":     map[string]any{"type": "boolean"},
						"hasMoons":        map[string]any{"type": "boolean"},
						"issue":           map[string]any{"type": "object"},
						"planets":         map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"resources":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"buildings":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"research":        map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"fleet":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"defense":         map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planetId", "commanderActive", "planetType", "moonEnabled", "hasMoons", "planets", "resources", "buildings", "research", "fleet", "defense"},
				},
			},
			"required": []string{"empire"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func technologyTreeTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_technology_tree",
		Title:       "Get Technology Tree",
		Description: "Return the read-only legacy technology tree for the current planet, with optional requirements detail and info table for one technology id.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
				"detailsId": map[string]any{
					"type":        "integer",
					"description": "Optional technology id for the multi-step requirements detail tree.",
					"minimum":     0,
				},
				"infoId": map[string]any{
					"type":        "integer",
					"description": "Optional technology id for the legacy info/details table.",
					"minimum":     0,
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"technology": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"planetId": map[string]any{"type": "integer"},
						"groups":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"details":  map[string]any{"type": "object"},
						"info":     map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "groups"},
				},
			},
			"required": []string{"technology"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func buildingOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_building_options",
		Title:       "Get Building Options",
		Description: "Return read-only legacy building queue and build options for the authenticated player's current or selected planet without finishing queues or updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"buildingOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planet":          map[string]any{"type": "object"},
						"commanderActive": map[string]any{"type": "boolean"},
						"queue":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"items":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "commanderActive", "queue", "items"},
				},
			},
			"required": []string{"buildingOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func researchOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_research_options",
		Title:       "Get Research Options",
		Description: "Return read-only legacy research queue and research options for the authenticated player's current or selected planet without finishing queues or updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"researchOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId": map[string]any{"type": "integer"},
						"planet":   map[string]any{"type": "object"},
						"hasLab":   map[string]any{"type": "boolean"},
						"active":   map[string]any{"type": "object"},
						"items":    map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "hasLab", "items"},
				},
			},
			"required": []string{"researchOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func shipyardOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_shipyard_options",
		Title:       "Get Shipyard Options",
		Description: "Return read-only legacy shipyard queue and ship build options for the authenticated player's current or selected planet without finishing queues or updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"shipyardOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planet":          map[string]any{"type": "object"},
						"commanderActive": map[string]any{"type": "boolean"},
						"hasShipyard":     map[string]any{"type": "boolean"},
						"busy":            map[string]any{"type": "boolean"},
						"queue":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"items":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "commanderActive", "hasShipyard", "busy", "queue", "items"},
				},
			},
			"required": []string{"shipyardOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func defenseOptionsTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "get_defense_options",
		Title:       "Get Defense Options",
		Description: "Return read-only legacy defense queue and defense build options for the authenticated player's current or selected planet without finishing queues or updating resources.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet context.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"defenseOptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planet":          map[string]any{"type": "object"},
						"commanderActive": map[string]any{"type": "boolean"},
						"hasShipyard":     map[string]any{"type": "boolean"},
						"busy":            map[string]any{"type": "boolean"},
						"queue":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"items":           map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					},
					"required": []string{"playerId", "planet", "commanderActive", "hasShipyard", "busy", "queue", "items"},
				},
			},
			"required": []string{"defenseOptions"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func validateFleetDispatchTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "validate_fleet_dispatch",
		Title:       "Validate Fleet Dispatch",
		Description: "Dry-run a fleet dispatch against the authenticated player's current fleet screen and return readiness, issue, fuel, cargo, and confirmation data.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId":       map[string]any{"type": "integer", "minimum": 1},
				"ships":          map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
				"resources":      map[string]any{"type": "object", "properties": map[string]any{"metal": map[string]any{"type": "integer", "minimum": 0}, "crystal": map[string]any{"type": "integer", "minimum": 0}, "deuterium": map[string]any{"type": "integer", "minimum": 0}}, "additionalProperties": false},
				"targetGalaxy":   map[string]any{"type": "integer", "minimum": 1},
				"targetSystem":   map[string]any{"type": "integer", "minimum": 1},
				"targetPosition": map[string]any{"type": "integer", "minimum": 1},
				"targetType":     map[string]any{"type": "integer", "minimum": 1},
				"mission":        map[string]any{"type": "integer", "minimum": 1},
				"speed":          map[string]any{"type": "integer", "minimum": 0},
				"holdHours":      map[string]any{"type": "integer", "minimum": 0},
				"expeditionHours": map[string]any{
					"type":    "integer",
					"minimum": 0,
				},
				"unionId": map[string]any{"type": "integer", "minimum": 0},
			},
			"required":             []string{"ships", "targetGalaxy", "targetSystem", "targetPosition", "targetType", "mission"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetDispatchValidation": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"ready":                map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"totalShips":           map[string]any{"type": "integer"},
						"mission":              map[string]any{"type": "integer"},
						"target":               map[string]any{"type": "object"},
						"targetType":           map[string]any{"type": "integer"},
						"speed":                map[string]any{"type": "integer"},
						"fuelConsumption":      map[string]any{"type": "integer"},
						"cargo":                map[string]any{"type": "integer"},
						"remainingCargo":       map[string]any{"type": "integer"},
						"durationSeconds":      map[string]any{"type": "integer"},
						"distance":             map[string]any{"type": "integer"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "ready", "dryRun", "requiresConfirmation", "totalShips", "mission", "target", "targetType"},
				},
			},
			"required": []string{"fleetDispatchValidation"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    true,
			"destructiveHint": false,
			"idempotentHint":  true,
		},
	}
}

func dispatchFleetTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "dispatch_fleet",
		Title:       "Dispatch Fleet",
		Description: "Execute a fleet dispatch only with the exact confirmation returned by validate_fleet_dispatch for the same arguments.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId":        map[string]any{"type": "integer", "minimum": 1},
				"ships":           map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}},
				"resources":       map[string]any{"type": "object", "properties": map[string]any{"metal": map[string]any{"type": "integer", "minimum": 0}, "crystal": map[string]any{"type": "integer", "minimum": 0}, "deuterium": map[string]any{"type": "integer", "minimum": 0}}, "additionalProperties": false},
				"targetGalaxy":    map[string]any{"type": "integer", "minimum": 1},
				"targetSystem":    map[string]any{"type": "integer", "minimum": 1},
				"targetPosition":  map[string]any{"type": "integer", "minimum": 1},
				"targetType":      map[string]any{"type": "integer", "minimum": 1},
				"mission":         map[string]any{"type": "integer", "minimum": 1},
				"speed":           map[string]any{"type": "integer", "minimum": 0},
				"holdHours":       map[string]any{"type": "integer", "minimum": 0},
				"expeditionHours": map[string]any{"type": "integer", "minimum": 0},
				"unionId":         map[string]any{"type": "integer", "minimum": 0},
				"confirm":         map[string]any{"type": "string"},
			},
			"required":             []string{"ships", "targetGalaxy", "targetSystem", "targetPosition", "targetType", "mission", "confirm"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetDispatch": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":        map[string]any{"type": "integer"},
						"planetId":        map[string]any{"type": "integer"},
						"ready":           map[string]any{"type": "boolean"},
						"dryRun":          map[string]any{"type": "boolean"},
						"executed":        map[string]any{"type": "boolean"},
						"totalShips":      map[string]any{"type": "integer"},
						"mission":         map[string]any{"type": "integer"},
						"target":          map[string]any{"type": "object"},
						"targetType":      map[string]any{"type": "integer"},
						"confirmation":    map[string]any{"type": "string"},
						"fuelConsumption": map[string]any{"type": "integer"},
						"issue":           map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "ready", "dryRun", "executed", "totalShips", "mission", "target", "targetType"},
				},
			},
			"required": []string{"fleetDispatch"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func recallFleetTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "recall_fleet",
		Title:       "Recall Fleet",
		Description: "Recall one owned outgoing fleet. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fleetId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned fleet id to recall.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same fleetId.",
				},
			},
			"required":             []string{"fleetId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recallFleet": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"fleetId":              map[string]any{"type": "integer"},
						"ownerId":              map[string]any{"type": "integer"},
						"mission":              map[string]any{"type": "integer"},
						"totalShips":           map[string]any{"type": "integer"},
						"recallable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "fleetId", "recallable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"recallFleet"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func scanPhalanxTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "scan_phalanx",
		Title:       "Scan Sensor Phalanx",
		Description: "Dry-run or confirm a legacy sensor phalanx scan. Confirmed scans spend deuterium and return visible fleet movements.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Owned source moon id. Omit or pass 0 to use the active context.",
				},
				"targetPlanetId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Target planet id to scan.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by dry-run for the same source and target.",
				},
			},
			"required":             []string{"targetPlanetId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"phalanxScan": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"targetPlanetId":       map[string]any{"type": "integer"},
						"commander":            map[string]any{"type": "string"},
						"source":               map[string]any{"type": "object"},
						"target":               map[string]any{"type": "object"},
						"cost":                 map[string]any{"type": "integer"},
						"remainingDeuterium":   map[string]any{"type": "number"},
						"events":               map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "targetPlanetId", "source", "target", "cost", "remainingDeuterium", "events", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"phalanxScan"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func jumpGateTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "jump_gate",
		Title:       "Jump Gate",
		Description: "Dry-run or confirm a legacy jump gate move between owned moons. Confirmed jumps move ships and start gate cooldown.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Current planet or moon id. Omit or pass 0 to use the active context.",
				},
				"sourceMoonId": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"description": "Owned source moon id. Omit or pass 0 to use planetId.",
				},
				"targetMoonId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Owned target moon id.",
				},
				"ships": map[string]any{
					"type":                 "object",
					"description":          "Ship counts keyed by fleet technology id.",
					"additionalProperties": map[string]any{"type": "integer", "minimum": 0},
				},
				"dryRun":  map[string]any{"type": "boolean", "description": "Defaults to true. Set false only with the returned confirmation."},
				"confirm": map[string]any{"type": "string", "description": "Confirmation returned by dry-run."},
			},
			"required":             []string{"targetMoonId", "ships"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"jumpGate": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"sourceMoonId":         map[string]any{"type": "integer"},
						"targetMoonId":         map[string]any{"type": "integer"},
						"ships":                map[string]any{"type": "object"},
						"totalShips":           map[string]any{"type": "integer"},
						"status":               map[string]any{"type": "object"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "sourceMoonId", "targetMoonId", "ships", "totalShips", "status", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"jumpGate"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": false,
			"idempotentHint":  false,
		},
	}
}

func cancelBuildingQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "cancel_building_queue",
		Title:       "Cancel Building Queue",
		Description: "Cancel one owned building queue row. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Planet id. Omit to use the active planet.",
				},
				"listId": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Building queue list id to cancel.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planetId/listId.",
				},
			},
			"required":             []string{"listId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cancelBuildingQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"listId":               map[string]any{"type": "integer"},
						"techId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"level":                map[string]any{"type": "integer"},
						"destroy":              map[string]any{"type": "boolean"},
						"cancelable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "listId", "cancelable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"cancelBuildingQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func cancelResearchQueueTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "cancel_research_queue",
		Title:       "Cancel Research Queue",
		Description: "Cancel the active owned research queue. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the active research queue.",
				},
			},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cancelResearchQueue": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"taskId":               map[string]any{"type": "integer"},
						"techId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"level":                map[string]any{"type": "integer"},
						"cancelable":           map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "cancelable", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"cancelResearchQueue"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func enqueueShipyardOrderTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "enqueue_shipyard_order",
		Title:       "Enqueue Shipyard Order",
		Description: "Queue fleet or defense construction for an owned planet. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"kind": map[string]any{
					"type":        "string",
					"enum":        []string{"fleet", "defense"},
					"description": "Order catalog. Defaults to fleet.",
				},
				"itemId": map[string]any{
					"type":        "integer",
					"description": "Fleet or defense technology id to queue.",
				},
				"amount": map[string]any{
					"type":        "integer",
					"description": "Requested unit count. The game may clamp to order cap/resources.",
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet, kind, item, and amount.",
				},
			},
			"required":             []string{"itemId", "amount"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"enqueueShipyardOrder": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"kind":                 map[string]any{"type": "string"},
						"itemId":               map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"requested":            map[string]any{"type": "integer"},
						"amount":               map[string]any{"type": "integer"},
						"maxBuild":             map[string]any{"type": "integer"},
						"durationSeconds":      map[string]any{"type": "integer"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "kind", "itemId", "requested", "amount", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"enqueueShipyardOrder"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func updateResourceProductionTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "update_resource_production",
		Title:       "Update Resource Production",
		Description: "Update owned planet resource production percentages. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"production": map[string]any{
					"type":        "object",
					"description": "Producer id to percent, for example {\"1\":80,\"2\":70}. Supported ids are legacy resource producer ids.",
					"additionalProperties": map[string]any{
						"type":    "integer",
						"minimum": 0,
						"maximum": 100,
					},
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet and production map.",
				},
			},
			"required":             []string{"production"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"updateResourceProduction": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"settings":             map[string]any{"type": "array"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "settings", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"updateResourceProduction"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}

func recruitOfficerTool() domainmcp.Tool {
	return domainmcp.Tool{
		Name:        "recruit_officer",
		Title:       "Recruit Officer",
		Description: "Recruit or extend a commander/officer using Dark Matter. Defaults to dry-run and requires the returned confirmation string before execution.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"planetId": map[string]any{
					"type":        "integer",
					"description": "Owned planet id. Omit or pass 0 to use the active planet.",
				},
				"officerId": map[string]any{
					"type":        "integer",
					"description": "Legacy officer id: 1 commander, 2 admiral, 3 engineer, 4 geologist, 5 technocrat.",
					"minimum":     1,
					"maximum":     5,
				},
				"days": map[string]any{
					"type":        "integer",
					"description": "Recruitment duration in days. Defaults to 7; allowed values are 7 and 90.",
					"enum":        []int{7, 90},
				},
				"dryRun": map[string]any{
					"type":        "boolean",
					"description": "Defaults to true. Set false only with a matching confirmation value.",
				},
				"confirm": map[string]any{
					"type":        "string",
					"description": "Exact confirmation string returned by a dry-run for the same planet, officer, and duration.",
				},
			},
			"required":             []string{"officerId"},
			"additionalProperties": false,
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recruitOfficer": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"playerId":             map[string]any{"type": "integer"},
						"planetId":             map[string]any{"type": "integer"},
						"officerId":            map[string]any{"type": "integer"},
						"name":                 map[string]any{"type": "string"},
						"days":                 map[string]any{"type": "integer"},
						"cost":                 map[string]any{"type": "integer"},
						"paidDarkMatter":       map[string]any{"type": "integer"},
						"freeDarkMatter":       map[string]any{"type": "integer"},
						"until":                map[string]any{"type": "integer"},
						"daysLeft":             map[string]any{"type": "integer"},
						"active":               map[string]any{"type": "boolean"},
						"dryRun":               map[string]any{"type": "boolean"},
						"requiresConfirmation": map[string]any{"type": "boolean"},
						"confirmation":         map[string]any{"type": "string"},
						"executed":             map[string]any{"type": "boolean"},
						"issue":                map[string]any{"type": "object"},
					},
					"required": []string{"playerId", "planetId", "officerId", "days", "cost", "paidDarkMatter", "freeDarkMatter", "active", "dryRun", "requiresConfirmation", "executed"},
				},
			},
			"required": []string{"recruitOfficer"},
		},
		Annotations: map[string]any{
			"readOnlyHint":    false,
			"destructiveHint": true,
			"idempotentHint":  false,
		},
	}
}
