-- Supporting records for employee work submissions and securely stored files.
ALTER TABLE task_comments
    ADD COLUMN parent_comment_id UUID REFERENCES task_comments(id) ON DELETE RESTRICT,
    ADD CONSTRAINT chk_comment_not_own_parent
        CHECK (parent_comment_id IS NULL OR parent_comment_id <> id);
CREATE INDEX idx_task_comments_parent ON task_comments(parent_comment_id);

CREATE TABLE task_submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE RESTRICT,
    submitted_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    summary TEXT NOT NULL CHECK (length(btrim(summary)) BETWEEN 1 AND 10000),
    reference_links JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_submission_links_array CHECK (jsonb_typeof(reference_links) = 'array')
);
CREATE INDEX idx_task_submissions_task ON task_submissions(task_id, created_at);

CREATE TABLE task_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE RESTRICT,
    uploaded_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    storage_key UUID NOT NULL UNIQUE,
    original_name VARCHAR(255) NOT NULL,
    media_type VARCHAR(120) NOT NULL,
    byte_size BIGINT NOT NULL CHECK (byte_size BETWEEN 1 AND 10485760),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_task_attachments_task ON task_attachments(task_id, created_at);

CREATE TABLE task_submission_attachments (
    submission_id UUID NOT NULL REFERENCES task_submissions(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES task_attachments(id) ON DELETE RESTRICT,
    PRIMARY KEY (submission_id, attachment_id)
);
