package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (r *ConversationRepository) Create(ctx context.Context, actor string, name *string, members []PrincipalRef, reuse bool) (Conversation, bool, error) {
	var result Conversation
	name, err := normalizedName(name)
	if err != nil {
		return result, false, err
	}
	if (reuse && name != nil) || (!reuse && name == nil) || len(members) > 1000 {
		return result, false, ErrConversationInput
	}
	actorRef, err := NormalizePrincipal(PrincipalRef{"human", actor})
	if err != nil {
		return result, false, err
	}
	refs := []PrincipalRef{actorRef}
	seen := map[PrincipalRef]bool{}
	for _, p := range members {
		p, err = NormalizePrincipal(p)
		if err != nil || seen[p] {
			return result, false, ErrConversationInput
		}
		seen[p] = true
		if p != actorRef {
			refs = append(refs, p)
		}
	}
	if len(refs) < 2 {
		return result, false, ErrConversationInput
	}
	sortRefs(refs)
	created := false
	err = r.write(ctx, func(tx *sql.Tx) error {
		created = false
		if err := lockSets(ctx, tx, refs); err != nil {
			return err
		}
		if err := selectedExist(ctx, tx, refs); err != nil {
			return err
		}
		id := ""
		if reuse {
			keys := make([]string, len(refs))
			for i, p := range refs {
				keys[i] = principalKey(p)
			}
			// Exact current set, not creation set; hash is only a lock, never identity.
			err := tx.QueryRowContext(ctx, `SELECT c.id FROM conversations c
     WHERE c.name IS NULL
     AND (SELECT count(*) FROM conversation_members m WHERE m.conversation_id=c.id)=$1
     AND NOT EXISTS(SELECT 1 FROM conversation_members m WHERE m.conversation_id=c.id
       AND NOT (m.member_kind||':'||coalesce(m.human_sub,m.bot_id::text)=ANY($2::text[])))
     ORDER BY c.created_at,c.id LIMIT 1 FOR UPDATE OF c`, len(refs), keys).Scan(&id)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if id != "" {
				// Recheck after row lock as mutations may have completed while waiting.
				current, err := memberRefs(ctx, tx, id)
				if err != nil {
					return err
				}
				var currentName *string
				if err = tx.QueryRowContext(ctx, "SELECT name FROM conversations WHERE id=$1", id).Scan(&currentName); err != nil {
					return err
				}
				if currentName != nil || !sameRefs(current, refs) {
					return errConversationRetry
				}
			}
		}
		if id == "" {
			if err := tx.QueryRowContext(ctx, `INSERT INTO conversations(name,created_by_kind,created_by_id)
       VALUES($1,'human',$2) RETURNING id`, name, actor).Scan(&id); err != nil {
				return err
			}
			for _, p := range refs {
				if err := insertMember(ctx, tx, id, p); err != nil {
					return err
				}
			}
			created = true
		}
		var err error
		result, err = conversationView(ctx, tx, actor, &id)
		return err
	})
	return result, created, err
}

// A mutation locks old/new typed sets in stable order, then locks the conversation,
// re-reads the set and retries a stale snapshot. It never locks a stale row first.
func (r *ConversationRepository) mutate(ctx context.Context, actor, id string, target *PrincipalRef, add bool, apply func(*sql.Tx, []PrincipalRef) error) error {
	if _, err := ParseChannel(id); err != nil {
		return err
	}
	return r.write(ctx, func(tx *sql.Tx) error {
		old, err := memberRefs(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = requireHuman(old, actor); err != nil {
			return err
		}
		next := append([]PrincipalRef{}, old...)
		if target != nil {
			if add && !includes(next, *target) {
				next = append(next, *target)
			}
			if !add {
				next = nil
				for _, p := range old {
					if p != *target {
						next = append(next, p)
					}
				}
			}
		}
		sortRefs(next)
		if err = lockSets(ctx, tx, old, next); err != nil {
			return err
		}
		if err = lockConversation(ctx, tx, id); err != nil {
			return err
		}
		current, err := memberRefs(ctx, tx, id)
		if err != nil {
			return err
		}
		if !sameRefs(current, old) {
			return errConversationRetry
		}
		if err = requireHuman(current, actor); err != nil {
			return err
		}
		return apply(tx, current)
	})
}
func (r *ConversationRepository) Rename(ctx context.Context, actor, id string, name *string) (Conversation, error) {
	var result Conversation
	name, err := normalizedName(name)
	if err != nil {
		return result, err
	}
	err = r.mutate(ctx, actor, id, nil, false, func(tx *sql.Tx, _ []PrincipalRef) error {
		if _, err := tx.ExecContext(ctx, "UPDATE conversations SET name=$2 WHERE id=$1", id, name); err != nil {
			return err
		}
		var err error
		result, err = conversationView(ctx, tx, actor, &id)
		return err
	})
	return result, err
}
func (r *ConversationRepository) ChangeMember(ctx context.Context, actor, id string, target PrincipalRef, add bool) error {
	var err error
	target, err = NormalizePrincipal(target)
	if err != nil {
		return err
	}
	return r.mutate(ctx, actor, id, &target, add, func(tx *sql.Tx, current []PrincipalRef) error {
		if add {
			if includes(current, target) {
				return nil
			}
			if err := selectedExist(ctx, tx, []PrincipalRef{target}); err != nil {
				return err
			}
			return insertMember(ctx, tx, id, target)
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM conversation_members WHERE conversation_id=$1
       AND member_kind=$2 AND coalesce(human_sub,bot_id::text)=$3`, id, target.Kind, target.ID)
		return err
	})
}
func stateAccess(ctx context.Context, tx *sql.Tx, actor string, channel *string) error {
	if channel == nil {
		return nil
	}
	if _, err := ParseChannel(*channel); err != nil {
		return err
	}
	if err := lockConversation(ctx, tx, *channel); err != nil {
		return err
	}
	refs, err := memberRefs(ctx, tx, *channel)
	if err != nil {
		return err
	}
	return requireHuman(refs, actor)
}
func (r *ConversationRepository) SetMute(ctx context.Context, actor string, channel *string, muted bool) error {
	return r.write(ctx, func(tx *sql.Tx) error {
		if err := stateAccess(ctx, tx, actor, channel); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO conversation_user_state(user_sub,channel_id,muted)
      VALUES($1,$2,$3) ON CONFLICT ON CONSTRAINT conversation_user_state_key DO UPDATE SET muted=EXCLUDED.muted`, actor, channel, muted)
		return err
	})
}
func (r *ConversationRepository) MarkRead(ctx context.Context, actor string, channel *string, messageID string) error {
	if _, err := ParseChannel(messageID); err != nil {
		return err
	}
	return r.write(ctx, func(tx *sql.Tx) error {
		if err := stateAccess(ctx, tx, actor, channel); err != nil {
			return err
		}
		var at time.Time
		err := tx.QueryRowContext(ctx, `SELECT created_at FROM messages
      WHERE id=$1 AND channel_id IS NOT DISTINCT FROM $2::uuid AND created_at<=CURRENT_TIMESTAMP`, messageID, channel).Scan(&at)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConversationInput
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO conversation_user_state(user_sub,channel_id,read_at,read_message_id)
      VALUES($1,$2,$3,$4) ON CONFLICT ON CONSTRAINT conversation_user_state_key
      DO UPDATE SET read_at=EXCLUDED.read_at,read_message_id=EXCLUDED.read_message_id
      WHERE conversation_user_state.read_at IS NULL OR
       (conversation_user_state.read_at,conversation_user_state.read_message_id)<(EXCLUDED.read_at,EXCLUDED.read_message_id)`, actor, channel, at, messageID)
		return err
	})
}
