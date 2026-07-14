package game

import (
	"context"
	"errors"

	apppublicsite "github.com/hjyoon/ogame-opensource/backend/internal/application/publicsite"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainpublicsite "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type AdminRepository interface {
	GetAdmin(context.Context, AdminQuery) (domaingame.Admin, error)
	MutateAdmin(context.Context, AdminMutationQuery) (*domaingame.AdminActionIssue, error)
}

type AdminCouponMailer interface {
	SendAdminCoupon(context.Context, domaingame.AdminCouponMail) error
}

type AdminReactivationMailer interface {
	SendAdminReactivation(context.Context, domaingame.AdminReactivationMail) error
}

type AdminBotEditRepository interface {
	MutateAdminBotEdit(context.Context, AdminBotEditMutationQuery) (AdminBotEditMutationResult, error)
}

type AdminQuery struct {
	PlayerID       int
	PlanetID       int
	Mode           string
	TargetPlayerID int
	TargetPlanetID int
	Filter         string
	LoginName      string
	LoginUserID    int
	LoginIP        string
	LoginUserIDSet bool
	UserLogSearch  *domaingame.AdminUserLogSearch
	LocaSource     string
	LocaTarget     string
	CouponFrom     int
	PlanetSearch   *domaingame.AdminPlanetSearch
}

type AdminCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mode            string
	TargetPlayerID  int
	TargetPlanetID  int
	Filter          string
	LoginName       string
	LoginUserID     int
	LoginIP         string
	LoginUserIDSet  bool
	UserLogSearch   *domaingame.AdminUserLogSearch
	LocaSource      string
	LocaTarget      string
	CouponFrom      int
	PlanetSearch    *domaingame.AdminPlanetSearch
}

type AdminMutationQuery struct {
	PlayerID      int
	PlanetID      int
	RemoteAddr    string
	Mode          string
	Action        string
	TaskID        int
	TargetIDs     []int
	BanMode       int
	Days          int
	Hours         int
	Reason        string
	Values        map[string]int
	User          *domaingame.AdminUserMutation
	Planet        *domaingame.AdminPlanetMutation
	PlanetSearch  *domaingame.AdminPlanetSearch
	Universe      *domaingame.AdminUniverseMutation
	Category      int
	Subject       string
	Text          string
	ReportIDs     []int
	DeleteMode    string
	FileName      string
	Amount        int
	ItemID        int
	DayMonth      string
	HourMinute    string
	InactiveDays  int
	IngameDays    int
	PeriodicDays  int
	ModName       string
	Name          string
	Filter        string
	UserLogSearch *domaingame.AdminUserLogSearch
}

type AdminMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Mode            string
	TargetPlayerID  int
	TargetPlanetID  int
	Filter          string
	CouponFrom      int
	LoginName       string
	LoginUserID     int
	LoginIP         string
	LoginUserIDSet  bool
	LocaSource      string
	LocaTarget      string
	Action          string
	TaskID          int
	TargetIDs       []int
	BanMode         int
	Days            int
	Hours           int
	Reason          string
	Values          map[string]int
	User            *domaingame.AdminUserMutation
	Planet          *domaingame.AdminPlanetMutation
	PlanetSearch    *domaingame.AdminPlanetSearch
	Universe        *domaingame.AdminUniverseMutation
	Category        int
	Subject         string
	Text            string
	ReportIDs       []int
	DeleteMode      string
	FileName        string
	Amount          int
	ItemID          int
	DayMonth        string
	HourMinute      string
	InactiveDays    int
	IngameDays      int
	PeriodicDays    int
	ModName         string
	Name            string
	UserLogSearch   *domaingame.AdminUserLogSearch
}

type AdminBotEditMutationQuery struct {
	PlayerID   int
	Action     string
	StrategyID int
	Name       string
	Source     string
}

type AdminBotEditMutationCommand struct {
	PublicSession   string
	PrivateSessions map[string]string
	RemoteAddr      string
	PlanetID        int
	Action          string
	StrategyID      int
	Name            string
	Source          string
}

type AdminBotEditMutationResult struct {
	Authenticated      bool
	Issues             []domainpublicsite.SessionIssue
	ActionIssue        *domaingame.AdminActionIssue
	Source             string
	Name               string
	Strategies         []domaingame.AdminBotStrategy
	SelectedStrategyID int
}

type AdminResult struct {
	Authenticated bool
	Issues        []domainpublicsite.SessionIssue
	Admin         domaingame.Admin
	ActionIssue   *domaingame.AdminActionIssue
}

type AdminService struct {
	sessions         SessionLookup
	repository       AdminRepository
	couponMail       AdminCouponMailer
	reactivationMail AdminReactivationMailer
}

func NewAdminService(sessions SessionLookup, repository AdminRepository) AdminService {
	return AdminService{sessions: sessions, repository: repository}
}

func NewAdminServiceWithCouponMailer(sessions SessionLookup, repository AdminRepository, mailer AdminCouponMailer) AdminService {
	return AdminService{sessions: sessions, repository: repository, couponMail: mailer}
}

func NewAdminServiceWithMailers(sessions SessionLookup, repository AdminRepository, couponMailer AdminCouponMailer, reactivationMailer AdminReactivationMailer) AdminService {
	return AdminService{sessions: sessions, repository: repository, couponMail: couponMailer, reactivationMail: reactivationMailer}
}

func (s AdminService) GetAdmin(ctx context.Context, command AdminCommand) (AdminResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AdminResult{}, errors.New("admin dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AdminResult{}, err
	}
	if !session.Authenticated {
		return AdminResult{Authenticated: false, Issues: session.Issues}, nil
	}
	admin, err := s.repository.GetAdmin(ctx, AdminQuery{
		PlayerID:       session.Session.PlayerID,
		PlanetID:       command.PlanetID,
		Mode:           command.Mode,
		TargetPlayerID: command.TargetPlayerID,
		TargetPlanetID: command.TargetPlanetID,
		Filter:         command.Filter,
		LoginName:      command.LoginName,
		LoginUserID:    command.LoginUserID,
		LoginIP:        command.LoginIP,
		LoginUserIDSet: command.LoginUserIDSet,
		UserLogSearch:  command.UserLogSearch,
		LocaSource:     command.LocaSource,
		LocaTarget:     command.LocaTarget,
		CouponFrom:     command.CouponFrom,
		PlanetSearch:   command.PlanetSearch,
	})
	if err != nil {
		return AdminResult{}, err
	}
	var issue *domaingame.AdminActionIssue
	if !admin.CanAccessMode() {
		issue = domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)
	}
	return AdminResult{Authenticated: true, Admin: admin, ActionIssue: issue}, nil
}

func (s AdminService) MutateAdmin(ctx context.Context, command AdminMutationCommand) (AdminResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AdminResult{}, errors.New("admin dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AdminResult{}, err
	}
	if !session.Authenticated {
		return AdminResult{Authenticated: false, Issues: session.Issues}, nil
	}
	admin, err := s.repository.GetAdmin(ctx, AdminQuery{
		PlayerID:       session.Session.PlayerID,
		PlanetID:       command.PlanetID,
		Mode:           command.Mode,
		TargetPlayerID: command.TargetPlayerID,
		TargetPlanetID: command.TargetPlanetID,
		Filter:         command.Filter,
		LoginName:      command.LoginName,
		LoginUserID:    command.LoginUserID,
		LoginIP:        command.LoginIP,
		LoginUserIDSet: command.LoginUserIDSet,
		LocaSource:     command.LocaSource,
		LocaTarget:     command.LocaTarget,
		CouponFrom:     command.CouponFrom,
	})
	if err != nil {
		return AdminResult{}, err
	}
	if !admin.CanAccessMode() {
		return AdminResult{Authenticated: true, Admin: admin, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)}, nil
	}
	if !admin.CanMutate(command.Action) {
		return AdminResult{Authenticated: true, Admin: admin, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)}, nil
	}
	issue, err := s.repository.MutateAdmin(ctx, AdminMutationQuery{
		PlayerID:      session.Session.PlayerID,
		PlanetID:      command.PlanetID,
		RemoteAddr:    command.RemoteAddr,
		Mode:          admin.Mode,
		Action:        command.Action,
		TaskID:        command.TaskID,
		TargetIDs:     command.TargetIDs,
		BanMode:       command.BanMode,
		Days:          command.Days,
		Hours:         command.Hours,
		Reason:        command.Reason,
		Values:        command.Values,
		User:          command.User,
		Planet:        command.Planet,
		PlanetSearch:  command.PlanetSearch,
		Universe:      command.Universe,
		Category:      command.Category,
		Subject:       command.Subject,
		Text:          command.Text,
		ReportIDs:     command.ReportIDs,
		DeleteMode:    command.DeleteMode,
		FileName:      command.FileName,
		Amount:        command.Amount,
		ItemID:        command.ItemID,
		DayMonth:      command.DayMonth,
		HourMinute:    command.HourMinute,
		InactiveDays:  command.InactiveDays,
		IngameDays:    command.IngameDays,
		PeriodicDays:  command.PeriodicDays,
		ModName:       command.ModName,
		Name:          command.Name,
		Filter:        command.Filter,
		UserLogSearch: command.UserLogSearch,
	})
	if err != nil {
		return AdminResult{}, err
	}
	if issue != nil && s.couponMail != nil {
		for _, message := range issue.OutboundCouponMails {
			if err := s.couponMail.SendAdminCoupon(ctx, message); err != nil {
				return AdminResult{}, err
			}
		}
	}
	if issue != nil && s.reactivationMail != nil {
		for _, message := range issue.OutboundReactivationMails {
			if err := s.reactivationMail.SendAdminReactivation(ctx, message); err != nil {
				return AdminResult{}, err
			}
		}
	}
	if issue != nil {
		issue.OutboundCouponMails = nil
		issue.OutboundReactivationMails = nil
	}
	reloadTargetPlanetID := command.TargetPlanetID
	if admin.Mode == "Planets" && issue != nil && issue.Result != nil && issue.Result.ItemID > 0 {
		reloadTargetPlanetID = issue.Result.ItemID
	}
	admin, err = s.repository.GetAdmin(ctx, AdminQuery{
		PlayerID:       session.Session.PlayerID,
		PlanetID:       command.PlanetID,
		Mode:           command.Mode,
		TargetPlayerID: command.TargetPlayerID,
		TargetPlanetID: reloadTargetPlanetID,
		Filter:         command.Filter,
		LoginName:      command.LoginName,
		LoginUserID:    command.LoginUserID,
		LoginIP:        command.LoginIP,
		LoginUserIDSet: command.LoginUserIDSet,
		UserLogSearch:  command.UserLogSearch,
		LocaSource:     command.LocaSource,
		LocaTarget:     command.LocaTarget,
		CouponFrom:     command.CouponFrom,
		PlanetSearch:   command.PlanetSearch,
	})
	if err != nil {
		return AdminResult{}, err
	}
	return AdminResult{Authenticated: true, Admin: admin, ActionIssue: issue}, nil
}

func (s AdminService) MutateAdminBotEdit(ctx context.Context, command AdminBotEditMutationCommand) (AdminBotEditMutationResult, error) {
	if s.sessions == nil || s.repository == nil {
		return AdminBotEditMutationResult{}, errors.New("admin dependencies unavailable")
	}
	session, err := s.sessions.GetGameSession(ctx, apppublicsite.GameSessionCommand{
		PublicSession:   command.PublicSession,
		PrivateSessions: command.PrivateSessions,
		RemoteAddr:      command.RemoteAddr,
	})
	if err != nil {
		return AdminBotEditMutationResult{}, err
	}
	if !session.Authenticated {
		return AdminBotEditMutationResult{Authenticated: false, Issues: session.Issues}, nil
	}
	admin, err := s.repository.GetAdmin(ctx, AdminQuery{
		PlayerID: session.Session.PlayerID,
		PlanetID: command.PlanetID,
		Mode:     "BotEdit",
	})
	if err != nil {
		return AdminBotEditMutationResult{}, err
	}
	if !admin.CanAccessMode() || !admin.CanMutate(command.Action) {
		return AdminBotEditMutationResult{Authenticated: true, ActionIssue: domaingame.AdminIssue(domaingame.AdminIssueAccessDenied)}, nil
	}
	repository, ok := s.repository.(AdminBotEditRepository)
	if !ok {
		return AdminBotEditMutationResult{}, errors.New("admin botedit mutation unavailable")
	}
	result, err := repository.MutateAdminBotEdit(ctx, AdminBotEditMutationQuery{
		PlayerID:   session.Session.PlayerID,
		Action:     command.Action,
		StrategyID: command.StrategyID,
		Name:       command.Name,
		Source:     command.Source,
	})
	if err != nil {
		return AdminBotEditMutationResult{}, err
	}
	result.Authenticated = true
	return result, nil
}
