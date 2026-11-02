
-- =========================================================
-- Office Task Manager: Initial Database Schema
-- PostgreSQL
-- =========================================================

-- 1. ENUM TYPES

CREATE TYPE user_role AS ENUM (
    'super_admin',
    'employee'
);

CREATE TYPE project_status AS ENUM (
    'planned',
    'active',
    'on_hold',
    'completed',
    'archived'
);

CREATE TYPE task_status AS ENUM (
    'todo',
    'in_progress',
    'in_review',
    'changes_requested',
    'approved',
    'blocked',
    'cancelled'
);

CREATE TYPE task_priority AS ENUM (
    'low',
    'medium',
    'high',
    'urgent'
);

CREATE TYPE assignee_work_status AS ENUM (
    'assigned',
    'in_progress',
    'ready_for_review'
);

CREATE TYPE task_review_decision AS ENUM (
    'approved',
    'changes_requested'
);


-- 2. USERS

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    full_name VARCHAR(150) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,

    role user_role NOT NULL DEFAULT 'employee',
    can_assign_tasks BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_super_admin_assignment
        CHECK (role <> 'super_admin' OR can_assign_tasks = TRUE)
);

-- Allow at most one Super Admin account.
CREATE UNIQUE INDEX uq_single_super_admin
    ON users (role)
    WHERE role = 'super_admin';


-- 3. PROJECTS

CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(200) NOT NULL,
    description TEXT,

    status project_status NOT NULL DEFAULT 'planned',

    start_date DATE,
    due_date DATE,

    created_by UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_project_dates
        CHECK (
            start_date IS NULL
            OR due_date IS NULL
            OR due_date >= start_date
        )
);


-- 4. PROJECT MEMBERS

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id)
        ON DELETE RESTRICT,

    user_id UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    removed_at TIMESTAMPTZ,

    PRIMARY KEY (project_id, user_id),

    CONSTRAINT chk_membership_dates
        CHECK (removed_at IS NULL OR removed_at >= joined_at)
);

CREATE INDEX idx_project_members_user
    ON project_members (user_id);


-- 5. TASKS AND NESTED SUBTASKS

CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    project_id UUID NOT NULL REFERENCES projects(id)
        ON DELETE RESTRICT,

    parent_task_id UUID,

    title VARCHAR(250) NOT NULL,
    description TEXT,

    status task_status NOT NULL DEFAULT 'todo',
    priority task_priority NOT NULL DEFAULT 'medium',

    created_by UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ,

    submitted_at TIMESTAMPTZ,
    approved_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Supports composite FK from task_assignees.
    CONSTRAINT uq_tasks_id_project UNIQUE (id, project_id),

    -- A task cannot be its own parent.
    CONSTRAINT chk_task_not_own_parent
        CHECK (parent_task_id IS NULL OR parent_task_id <> id),

    -- A subtask's parent must belong to the same project.
    CONSTRAINT fk_task_parent_same_project
        FOREIGN KEY (parent_task_id, project_id)
        REFERENCES tasks (id, project_id)
        ON DELETE RESTRICT,

    CONSTRAINT chk_task_dates
        CHECK (
            start_at IS NULL
            OR due_at IS NULL
            OR due_at >= start_at
        )
);

CREATE INDEX idx_tasks_project
    ON tasks (project_id);

CREATE INDEX idx_tasks_parent
    ON tasks (parent_task_id);

CREATE INDEX idx_tasks_status
    ON tasks (status);

CREATE INDEX idx_tasks_due_at
    ON tasks (due_at);


-- Prevent indirect circular parent-child relationships.
-- Example: A -> B -> C -> A is rejected.

CREATE FUNCTION prevent_task_cycle()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.parent_task_id IS NULL THEN
        RETURN NEW;
    END IF;

    IF EXISTS (
        WITH RECURSIVE ancestors(id, parent_task_id) AS (
            SELECT id, parent_task_id
            FROM tasks
            WHERE id = NEW.parent_task_id

            UNION

            SELECT t.id, t.parent_task_id
            FROM tasks t
            JOIN ancestors a
                ON t.id = a.parent_task_id
        )
        SELECT 1
        FROM ancestors
        WHERE id = NEW.id
    ) THEN
        RAISE EXCEPTION
            'Circular task hierarchy detected for task %',
            NEW.id;
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_prevent_task_cycle
BEFORE INSERT OR UPDATE OF parent_task_id
ON tasks
FOR EACH ROW
EXECUTE FUNCTION prevent_task_cycle();


-- 6. TASK ASSIGNEES

CREATE TABLE task_assignees (
    task_id UUID NOT NULL,
    project_id UUID NOT NULL,
    user_id UUID NOT NULL,

    assigned_by UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    work_status assignee_work_status NOT NULL DEFAULT 'assigned',

    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (task_id, user_id),

    CONSTRAINT fk_assignee_task_project
        FOREIGN KEY (task_id, project_id)
        REFERENCES tasks (id, project_id)
        ON DELETE RESTRICT,

    CONSTRAINT fk_assignee_project_member
        FOREIGN KEY (project_id, user_id)
        REFERENCES project_members (project_id, user_id)
        ON DELETE RESTRICT
);

CREATE INDEX idx_task_assignees_user
    ON task_assignees (user_id);

CREATE INDEX idx_task_assignees_project
    ON task_assignees (project_id);


-- Validate assignee membership and assignment permissions.

CREATE FUNCTION validate_task_assignment()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    assignee_active BOOLEAN;
    assigner_allowed BOOLEAN;
    member_active BOOLEAN;
BEGIN
    SELECT is_active
    INTO assignee_active
    FROM users
    WHERE id = NEW.user_id;

    IF assignee_active IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION 'Assignee must be an active user';
    END IF;

    SELECT is_active
       AND (role = 'super_admin' OR can_assign_tasks)
    INTO assigner_allowed
    FROM users
    WHERE id = NEW.assigned_by;

    IF assigner_allowed IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION
            'Only an active Super Admin or authorized task assigner can assign tasks';
    END IF;

    SELECT (removed_at IS NULL)
    INTO member_active
    FROM project_members
    WHERE project_id = NEW.project_id
      AND user_id = NEW.user_id;

    IF member_active IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION
            'Assignee must be an active member of the project';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_validate_task_assignment
BEFORE INSERT OR UPDATE OF task_id, project_id, user_id, assigned_by
ON task_assignees
FOR EACH ROW
EXECUTE FUNCTION validate_task_assignment();


-- 7. TASK COMMENTS

CREATE TABLE task_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL REFERENCES tasks(id)
        ON DELETE RESTRICT,

    user_id UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    comment TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_comments_task
    ON task_comments (task_id, created_at);


-- 8. TASK REVIEWS

CREATE TABLE task_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL REFERENCES tasks(id)
        ON DELETE RESTRICT,

    reviewer_id UUID NOT NULL REFERENCES users(id)
        ON DELETE RESTRICT,

    decision task_review_decision NOT NULL,
    feedback TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_reviews_task
    ON task_reviews (task_id, created_at);


-- Only the active Super Admin can review a submitted task.

CREATE FUNCTION validate_task_reviewer()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    reviewer_is_admin BOOLEAN;
    current_task_status task_status;
BEGIN
    SELECT (is_active AND role = 'super_admin')
    INTO reviewer_is_admin
    FROM users
    WHERE id = NEW.reviewer_id;

    IF reviewer_is_admin IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION
            'Only the active Super Admin can review tasks';
    END IF;

    SELECT status
    INTO current_task_status
    FROM tasks
    WHERE id = NEW.task_id;

    IF current_task_status IS DISTINCT FROM 'in_review' THEN
        RAISE EXCEPTION
            'Only tasks in review can receive a review decision';
    END IF;

    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_validate_task_reviewer
BEFORE INSERT
ON task_reviews
FOR EACH ROW
EXECUTE FUNCTION validate_task_reviewer();


-- 9. TASK ACTIVITY / AUDIT HISTORY

CREATE TABLE task_activity (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    task_id UUID NOT NULL REFERENCES tasks(id)
        ON DELETE RESTRICT,

    actor_id UUID REFERENCES users(id)
        ON DELETE RESTRICT,

    action VARCHAR(100) NOT NULL,

    old_values JSONB,
    new_values JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_activity_task
    ON task_activity (task_id, created_at);

CREATE INDEX idx_task_activity_actor
    ON task_activity (actor_id);


-- 10. AUTOMATIC updated_at TIMESTAMPS

CREATE FUNCTION set_updated_at()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_projects_updated_at
BEFORE UPDATE ON projects
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_task_comments_updated_at
BEFORE UPDATE ON task_comments
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
