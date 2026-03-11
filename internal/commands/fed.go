package commands

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/m4rcel-lol/cliverse/internal/models"
)

// HandleFed handles federation admin commands.
func HandleFed(ctx *Context) error {
	if !ctx.User.IsAdmin {
		return fmt.Errorf("\033[31maccess denied: admin only\033[0m")
	}

	if len(ctx.Args) == 0 {
		return fmt.Errorf("usage: fed <list|block|allow|remove|status> [DOMAIN]")
	}

	switch ctx.Args[0] {
	case "list":
		return fedList(ctx)
	case "block":
		return fedSetPolicy(ctx, models.PolicyBlock)
	case "allow":
		return fedSetPolicy(ctx, models.PolicyAllow)
	case "remove":
		return fedRemove(ctx)
	case "status":
		return fedStatus(ctx)
	default:
		return fmt.Errorf("unknown fed subcommand: %s", ctx.Args[0])
	}
}

func fedList(ctx *Context) error {
	policies, err := ctx.DB.ListDomainPolicies(ctx.Ctx)
	if err != nil {
		return fmt.Errorf("list policies: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[1mDomain Policies (%d):\033[0m\n", len(policies))
	if len(policies) == 0 {
		fmt.Fprintf(ctx.W, "No domain policies configured.\n\n")
		return nil
	}
	for _, p := range policies {
		actionColor := "\033[32m"
		if p.Action == models.PolicyBlock {
			actionColor = "\033[31m"
		} else if p.Action == models.PolicySilence {
			actionColor = "\033[33m"
		}
		reason := p.Reason
		if reason == "" {
			reason = "(no reason)"
		}
		fmt.Fprintf(ctx.W, "  %s%-8s\033[0m %s - %s\n", actionColor, p.Action, p.Domain, reason)
	}
	fmt.Fprintln(ctx.W)
	return nil
}

func fedSetPolicy(ctx *Context, action string) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: fed %s DOMAIN", ctx.Args[0])
	}

	domain := ctx.Args[1]
	reason := ""
	if len(ctx.Args) > 2 {
		reason = ctx.Args[2]
	}

	policy := &models.DomainPolicy{
		ID:        uuid.New(),
		Domain:    domain,
		Action:    action,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	if err := ctx.DB.CreateDomainPolicy(ctx.Ctx, policy); err != nil {
		return fmt.Errorf("set policy: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Domain policy set: %s → %s\033[0m\n", domain, action)
	return nil
}

func fedRemove(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: fed remove DOMAIN")
	}

	domain := ctx.Args[1]
	if err := ctx.DB.DeleteDomainPolicy(ctx.Ctx, domain); err != nil {
		return fmt.Errorf("remove policy: %w", err)
	}

	fmt.Fprintf(ctx.W, "\033[32m✓ Domain policy removed for %s\033[0m\n", domain)
	return nil
}

func fedStatus(ctx *Context) error {
	if len(ctx.Args) < 2 {
		return fmt.Errorf("usage: fed status DOMAIN")
	}

	domain := ctx.Args[1]
	policy, err := ctx.DB.GetDomainPolicy(ctx.Ctx, domain)
	if err != nil {
		return fmt.Errorf("get policy: %w", err)
	}

	if policy == nil {
		fmt.Fprintf(ctx.W, "%s: \033[32mno policy (allowed)\033[0m\n", domain)
		return nil
	}

	statusColor := "\033[32m"
	if policy.Action == models.PolicyBlock {
		statusColor = "\033[31m"
	} else if policy.Action == models.PolicySilence {
		statusColor = "\033[33m"
	}

	fmt.Fprintf(ctx.W, "%s: %s%s\033[0m", domain, statusColor, policy.Action)
	if policy.Reason != "" {
		fmt.Fprintf(ctx.W, " (%s)", policy.Reason)
	}
	fmt.Fprintln(ctx.W)
	return nil
}
