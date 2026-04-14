package ruleset

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

// ErrEmptyConfig означает, что загруженный файл не содержит правил.
var ErrEmptyConfig = errors.New("empty ruleset config")

// Repo загружает и отслеживает YAML-файл правил пересылки.
type Repo struct {
	logger  *slog.Logger
	cfg     config.RulesetConfig
	watcher *fsnotify.Watcher
}

// New создаёт новый экземпляр загрузчика правил.
func New(cfg config.RulesetConfig, logger *slog.Logger) *Repo {
	return &Repo{
		logger: logger.With("module", "repo.ruleset"),
		cfg:    cfg,
	}
}

// Load читает YAML-файл и возвращает валидированный RuleSet.
func (r *Repo) Load() (*domain.RuleSet, error) {
	data, err := os.ReadFile(r.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("read ruleset file: %w", err)
	}

	var rs domain.RuleSet
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return nil, fmt.Errorf("parse ruleset yaml: %w", err)
	}

	initialize(&rs)
	if err := validate(&rs); err != nil {
		return nil, err
	}
	transform(&rs)
	enrich(&rs)

	if err := check(&rs); err != nil {
		return &rs, err
	}

	return &rs, nil
}

// WatchContext запускает наблюдатель за изменениями файла.
func (r *Repo) WatchContext(ctx context.Context, onChange func()) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	r.watcher = w

	if err := w.Add(r.cfg.Path); err != nil {
		closeErr := w.Close()
		if closeErr != nil {
			return fmt.Errorf("watch file %q: %w (close: %v)", r.cfg.Path, err, closeErr)
		}
		return fmt.Errorf("watch file %q: %w", r.cfg.Path, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					r.logger.Info("Ruleset file changed", slog.String("file", r.cfg.Path))
					onChange()
				}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				r.logger.Error("Watcher error", slog.Any("error", err))
			}
		}
	}()

	return nil
}

// Close останавливает наблюдатель.
func (r *Repo) Close() error {
	if r.watcher != nil {
		return r.watcher.Close()
	}
	return nil
}

func initialize(rs *domain.RuleSet) {
	if rs.Sources == nil {
		rs.Sources = make(map[domain.ChatID]*domain.Source)
	}
	if rs.Destinations == nil {
		rs.Destinations = make(map[domain.ChatID]*domain.Destination)
	}
	if rs.ForwardRules == nil {
		rs.ForwardRules = make(map[domain.ForwardRuleID]*domain.ForwardRule)
	}
	rs.UniqueSources = make(map[domain.ChatID]struct{})
	rs.UniqueDestinations = make(map[domain.ChatID]struct{})
}

func validate(rs *domain.RuleSet) error {
	invalidID := regexp.MustCompile("[,:]")
	for id, rule := range rs.ForwardRules {
		if invalidID.MatchString(id) {
			return fmt.Errorf("forward rule id %q contains invalid characters ',' or ':'", id)
		}
		if rule.From < 0 {
			return fmt.Errorf("forward rule %q: From must be positive, got %d", id, rule.From)
		}
		for i, dst := range rule.To {
			if dst < 0 {
				return fmt.Errorf("forward rule %q: To[%d] must be positive, got %d", id, i, dst)
			}
			if rule.From == dst {
				return fmt.Errorf("forward rule %q: To[%d] must differ from From", id, i)
			}
		}
	}
	return nil
}

func transform(rs *domain.RuleSet) {
	// Инвертируем ключи Sources
	newSources := make(map[domain.ChatID]*domain.Source, len(rs.Sources))
	for id, src := range rs.Sources {
		newSources[-id] = src
	}
	rs.Sources = newSources

	// Инвертируем For-списки в Sources
	for _, src := range rs.Sources {
		negateChatIDs(src.Translate, src.Sign, src.Link, src.Prev, src.Next)
	}

	// Инвертируем ключи Destinations
	newDst := make(map[domain.ChatID]*domain.Destination, len(rs.Destinations))
	for id, dst := range rs.Destinations {
		newDst[-id] = dst
	}
	rs.Destinations = newDst

	// Инвертируем ID в ForwardRules
	for _, rule := range rs.ForwardRules {
		rule.From = -rule.From
		for i := range rule.To {
			rule.To[i] = -rule.To[i]
		}
		rule.Check = -rule.Check
		rule.Other = -rule.Other
	}
}

func enrich(rs *domain.RuleSet) {
	for id, dst := range rs.Destinations {
		dst.ChatID = id
	}
	for id, src := range rs.Sources {
		src.ChatID = id
	}

	ordered := make([]domain.ForwardRuleID, 0, len(rs.ForwardRules))
	for id, rule := range rs.ForwardRules {
		rule.ID = id
		srcID := rule.From
		if _, ok := rs.Sources[srcID]; !ok {
			rs.Sources[srcID] = &domain.Source{ChatID: srcID}
		}
		rs.UniqueSources[srcID] = struct{}{}
		for _, dstID := range rule.To {
			rs.UniqueDestinations[dstID] = struct{}{}
		}
		ordered = append(ordered, id)
	}
	rs.OrderedForwardRules = ordered
}

func check(rs *domain.RuleSet) error {
	if len(rs.UniqueSources) == 0 || len(rs.UniqueDestinations) == 0 || len(rs.OrderedForwardRules) == 0 {
		return ErrEmptyConfig
	}
	return nil
}

// negateChatIDs инвертирует ID в For-списках опций источника.
func negateChatIDs(opts ...any) {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		switch v := opt.(type) {
		case *domain.Translate:
			if v != nil {
				for i := range v.For {
					v.For[i] = -v.For[i]
				}
			}
		case *domain.Sign:
			if v != nil {
				for i := range v.For {
					v.For[i] = -v.For[i]
				}
			}
		case *domain.Link:
			if v != nil {
				for i := range v.For {
					v.For[i] = -v.For[i]
				}
			}
		case *domain.Prev:
			if v != nil {
				for i := range v.For {
					v.For[i] = -v.For[i]
				}
			}
		case *domain.Next:
			if v != nil {
				for i := range v.For {
					v.For[i] = -v.For[i]
				}
			}
		}
	}
}
