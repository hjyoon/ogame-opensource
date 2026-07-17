package mysqlgame

import (
	"context"
	"fmt"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

const adminCronDaySeconds = 24 * 60 * 60

type adminCronCouponRecipient struct {
	Character string
	Recipient string
	Language  string
}

func (r AdminRepository) finishDueAdminCouponCronTasks(ctx context.Context, tables adminCronTables, until int) ([]domaingame.AdminCouponMail, error) {
	tasks, err := r.loadAdminCouponCronTasks(ctx, tables.queue, until)
	if err != nil {
		return nil, err
	}
	mails := []domaingame.AdminCouponMail{}
	for _, task := range tasks {
		generated := []domaingame.AdminCouponMail{}
		claimed, err := finishDueQueueTaskAtomically(
			ctx,
			r.queryer,
			r.execer,
			tables.queue,
			dueQueueTaskClaim{TaskID: task.TaskID, Type: task.Type, End: task.End, AllowFrozen: true},
			until,
			func(queryer Queryer, execer Execer) error {
				transactional := r
				transactional.queryer = queryer
				transactional.execer = execer
				var finishErr error
				generated, finishErr = transactional.finishAdminCouponCronTask(ctx, tables, task)
				return finishErr
			},
		)
		if err != nil {
			return nil, err
		}
		if claimed {
			mails = append(mails, generated...)
		}
	}
	return mails, nil
}

func (r AdminRepository) finishAdminCouponCronTask(ctx context.Context, tables adminCronTables, task buildingQueueTask) ([]domaingame.AdminCouponMail, error) {
	inactiveDays := (task.ObjID >> 16) & 0xffff
	ingameDays := task.ObjID & 0xffff
	recipients, err := r.loadAdminCouponCronRecipients(
		ctx,
		tables.users,
		task.End-ingameDays*adminCronDaySeconds,
		task.End-inactiveDays*adminCronDaySeconds,
	)
	if err != nil {
		return nil, err
	}
	mails := make([]domaingame.AdminCouponMail, 0, len(recipients))
	for _, recipient := range recipients {
		code, err := r.insertAdminCoupon(ctx, task.SubID)
		if err != nil {
			return nil, err
		}
		mails = append(mails, domaingame.AdminCouponMail{
			Character: recipient.Character,
			Recipient: recipient.Recipient,
			Language:  recipient.Language,
			Code:      code,
		})
	}
	seconds := task.Level * adminCronDaySeconds
	if seconds > 0 {
		_, err = r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET end = end + ? WHERE task_id = ?", tables.queue), seconds, task.TaskID)
	} else {
		err = r.removeAdminCronTask(ctx, tables.queue, task.TaskID)
	}
	if err != nil {
		return nil, err
	}
	return mails, nil
}

func (r AdminRepository) loadAdminCouponCronTasks(ctx context.Context, queueTable string, until int) ([]buildingQueueTask, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT task_id, COALESCE(owner_id, 0), COALESCE(type, ''), COALESCE(sub_id, 0), COALESCE(obj_id, 0), COALESCE(level, 0), COALESCE(start, 0), COALESCE(end, 0), COALESCE(prio, 0), COALESCE(freeze, 0), COALESCE(frozen, 0) FROM %s WHERE end <= ? AND type = ? ORDER BY end ASC, prio DESC", queueTable), until, adminCouponQueueType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []buildingQueueTask{}
	for rows.Next() {
		var task buildingQueueTask
		if err := rows.Scan(&task.TaskID, &task.OwnerID, &task.Type, &task.SubID, &task.ObjID, &task.Level, &task.Start, &task.End, &task.Prio, &task.Freeze, &task.Frozen); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (r AdminRepository) loadAdminCouponCronRecipients(ctx context.Context, usersTable string, registeredBefore int, activeSince int) ([]adminCronCouponRecipient, error) {
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT COALESCE(oname, ''), COALESCE(pemail, ''), COALESCE(lang, '') FROM %s WHERE regdate < ? AND lastclick >= ?", usersTable), registeredBefore, activeSince)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	recipients := []adminCronCouponRecipient{}
	for rows.Next() {
		var recipient adminCronCouponRecipient
		if err := rows.Scan(&recipient.Character, &recipient.Recipient, &recipient.Language); err != nil {
			return nil, err
		}
		recipients = append(recipients, recipient)
	}
	return recipients, rows.Err()
}
