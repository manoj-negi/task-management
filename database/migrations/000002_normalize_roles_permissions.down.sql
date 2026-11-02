CREATE TYPE user_role AS ENUM ('super_admin', 'employee');
ALTER TABLE users ADD COLUMN role user_role;
UPDATE users u SET role = r.name::user_role FROM roles r WHERE r.id = u.role_id;
ALTER TABLE users ALTER COLUMN role SET NOT NULL;
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'employee';
ALTER TABLE users ADD COLUMN can_assign_tasks BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE users u SET can_assign_tasks = TRUE WHERE EXISTS (
    SELECT 1 FROM user_permissions up JOIN permissions p ON p.id = up.permission_id
    WHERE up.user_id = u.id AND p.code = 'assign_tasks'
) OR EXISTS (
    SELECT 1 FROM role_permissions rp JOIN permissions p ON p.id = rp.permission_id
    WHERE rp.role_id = u.role_id AND p.code = 'assign_tasks'
);
ALTER TABLE users ADD CONSTRAINT chk_super_admin_assignment CHECK (role <> 'super_admin' OR can_assign_tasks);
CREATE UNIQUE INDEX uq_single_super_admin ON users(role) WHERE role = 'super_admin';

DROP TRIGGER trg_single_super_admin ON users;
DROP FUNCTION enforce_single_super_admin();
CREATE OR REPLACE FUNCTION validate_task_assignment()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE assignee_active BOOLEAN; assigner_allowed BOOLEAN; member_active BOOLEAN;
BEGIN
    SELECT is_active INTO assignee_active FROM users WHERE id = NEW.user_id;
    IF assignee_active IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Assignee must be an active user'; END IF;
    SELECT is_active AND (role = 'super_admin' OR can_assign_tasks) INTO assigner_allowed FROM users WHERE id = NEW.assigned_by;
    IF assigner_allowed IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Only an active Super Admin or authorized task assigner can assign tasks'; END IF;
    SELECT removed_at IS NULL INTO member_active FROM project_members WHERE project_id = NEW.project_id AND user_id = NEW.user_id;
    IF member_active IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Assignee must be an active member of the project'; END IF;
    RETURN NEW;
END;
$$;
CREATE OR REPLACE FUNCTION validate_task_reviewer()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE reviewer_is_admin BOOLEAN; current_task_status task_status;
BEGIN
    SELECT (is_active AND role = 'super_admin') INTO reviewer_is_admin FROM users WHERE id = NEW.reviewer_id;
    IF reviewer_is_admin IS DISTINCT FROM TRUE THEN RAISE EXCEPTION 'Only the active Super Admin can review tasks'; END IF;
    SELECT status INTO current_task_status FROM tasks WHERE id = NEW.task_id;
    IF current_task_status IS DISTINCT FROM 'in_review' THEN RAISE EXCEPTION 'Only tasks in review can receive a review decision'; END IF;
    RETURN NEW;
END;
$$;

DROP INDEX idx_user_permissions_permission;
DROP INDEX idx_role_permissions_permission;
DROP INDEX idx_users_role_id;
ALTER TABLE users DROP COLUMN role_id;
DROP TABLE user_permissions;
DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;
