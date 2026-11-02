-- Normalize roles and permissions while preserving existing user access.
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(80) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_permissions (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, permission_id)
);

INSERT INTO roles (name, description) VALUES
    ('super_admin', 'Full administrative access'),
    ('employee', 'Standard employee access');

INSERT INTO permissions (code, description) VALUES
    ('manage_users', 'Create and manage employee accounts'),
    ('manage_projects', 'Create and manage projects and their members'),
    ('manage_tasks', 'Manage all tasks and assignments'),
    ('create_tasks', 'Create and assign tasks'),
    ('view_all_tasks', 'View all tasks'),
    ('view_all_projects', 'View all projects'),
    ('review_tasks', 'Review submitted tasks'),
    ('assign_tasks', 'Assign tasks within project membership');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name = 'super_admin';

ALTER TABLE users ADD COLUMN role_id UUID REFERENCES roles(id);
UPDATE users u SET role_id = r.id FROM roles r WHERE r.name = u.role::text;
ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN role_id DROP DEFAULT;

-- Preserve the per-user task-assigner setting for existing employees.
INSERT INTO user_permissions (user_id, permission_id)
SELECT u.id, p.id FROM users u CROSS JOIN permissions p
WHERE u.can_assign_tasks AND p.code = 'assign_tasks' AND u.role::text = 'employee';

DROP INDEX IF EXISTS uq_single_super_admin;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_super_admin_assignment;
ALTER TABLE users DROP COLUMN can_assign_tasks;
ALTER TABLE users DROP COLUMN role;
DROP TYPE user_role;

CREATE INDEX idx_users_role_id ON users(role_id);
CREATE INDEX idx_role_permissions_permission ON role_permissions(permission_id);
CREATE INDEX idx_user_permissions_permission ON user_permissions(permission_id);

-- Preserve the previous one-Super-Admin invariant after role becomes a FK.
CREATE FUNCTION enforce_single_super_admin()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE role_name TEXT;
BEGIN
    SELECT name INTO role_name FROM roles WHERE id = NEW.role_id;
    IF role_name = 'super_admin' THEN
        PERFORM pg_advisory_xact_lock(714205, 1);
        IF EXISTS (SELECT 1 FROM users WHERE role_id = NEW.role_id AND id <> NEW.id) THEN
            RAISE EXCEPTION 'Only one Super Admin account is allowed';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER trg_single_super_admin
BEFORE INSERT OR UPDATE OF role_id ON users
FOR EACH ROW EXECUTE FUNCTION enforce_single_super_admin();

CREATE OR REPLACE FUNCTION validate_task_assignment()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    assignee_active BOOLEAN;
    assigner_allowed BOOLEAN;
    member_active BOOLEAN;
BEGIN
    SELECT is_active INTO assignee_active FROM users WHERE id = NEW.user_id;
    IF assignee_active IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION 'Assignee must be an active user';
    END IF;

    SELECT u.is_active AND (
        r.name = 'super_admin' OR EXISTS (
            SELECT 1 FROM permissions p
            LEFT JOIN role_permissions rp ON rp.permission_id = p.id AND rp.role_id = u.role_id
            LEFT JOIN user_permissions up ON up.permission_id = p.id AND up.user_id = u.id
            WHERE p.code = 'assign_tasks' AND (rp.role_id IS NOT NULL OR up.user_id IS NOT NULL)
        )
    ) INTO assigner_allowed
    FROM users u JOIN roles r ON r.id = u.role_id WHERE u.id = NEW.assigned_by;
    IF assigner_allowed IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION 'Only an active authorized task assigner can assign tasks';
    END IF;

    SELECT removed_at IS NULL INTO member_active FROM project_members
    WHERE project_id = NEW.project_id AND user_id = NEW.user_id;
    IF member_active IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION 'Assignee must be an active member of the project';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION validate_task_reviewer()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    reviewer_allowed BOOLEAN;
    current_task_status task_status;
BEGIN
    SELECT u.is_active AND EXISTS (
        SELECT 1 FROM permissions p
        LEFT JOIN role_permissions rp ON rp.permission_id = p.id AND rp.role_id = u.role_id
        LEFT JOIN user_permissions up ON up.permission_id = p.id AND up.user_id = u.id
        WHERE p.code = 'review_tasks' AND (rp.role_id IS NOT NULL OR up.user_id IS NOT NULL)
    ) INTO reviewer_allowed FROM users u WHERE u.id = NEW.reviewer_id;
    IF reviewer_allowed IS DISTINCT FROM TRUE THEN
        RAISE EXCEPTION 'Reviewer must be active and have review_tasks permission';
    END IF;
    SELECT status INTO current_task_status FROM tasks WHERE id = NEW.task_id;
    IF current_task_status IS DISTINCT FROM 'in_review' THEN
        RAISE EXCEPTION 'Only tasks in review can receive a review decision';
    END IF;
    RETURN NEW;
END;
$$;
