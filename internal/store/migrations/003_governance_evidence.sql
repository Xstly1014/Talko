CREATE TABLE IF NOT EXISTS report_evidence(report_id VARCHAR(32) PRIMARY KEY,revision_id VARCHAR(32) NOT NULL,FOREIGN KEY(report_id) REFERENCES reports(id),FOREIGN KEY(revision_id) REFERENCES revisions(id));
CREATE TABLE IF NOT EXISTS case_resolutions(case_id VARCHAR(32) NOT NULL,kind VARCHAR(20) NOT NULL,action VARCHAR(20) NOT NULL,reason VARCHAR(500) NOT NULL,reviewer VARCHAR(254) NOT NULL,new_task_id VARCHAR(32) NULL,created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),PRIMARY KEY(kind,case_id));
SET @talko_migration_sql = IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='review_tasks' AND index_name='target_id' AND non_unique=0),'ALTER TABLE review_tasks DROP INDEX target_id, ADD INDEX idx_review_target(kind,target_id)','DO 0');
PREPARE talko_stmt FROM @talko_migration_sql;
EXECUTE talko_stmt;
DEALLOCATE PREPARE talko_stmt;
