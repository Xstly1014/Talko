INSERT IGNORE INTO audit_log(id,actor,action,target_id,reason)
SELECT MD5(CONCAT('migration004:',t.id)),'system:migration004','cancel_stale_task',t.id,'历史待审任务校正：对象已删除、下架、替换或不再处于待审状态'
FROM review_tasks t LEFT JOIN comments c ON t.kind='comment' AND c.id=t.target_id LEFT JOIN revisions r ON t.kind='post' AND r.id=t.target_id LEFT JOIN posts p ON p.id=COALESCE(r.post_id,c.post_id) LEFT JOIN profile_changes pc ON t.kind='profile' AND pc.id=t.target_id
WHERE t.status='pending' AND ((t.kind='comment' AND (c.status<>'pending' OR p.status IN ('hidden','deleted'))) OR (t.kind='post' AND (r.state<>'pending' OR p.status IN ('hidden','deleted') OR p.draft_revision_id<>r.id)) OR (t.kind='profile' AND pc.status<>'pending'));
UPDATE review_tasks t JOIN audit_log a ON a.target_id=t.id AND a.actor='system:migration004' SET t.status='cancelled' WHERE t.status='pending';
