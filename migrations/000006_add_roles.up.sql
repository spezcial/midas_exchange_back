ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('client', 'admin', 'super_admin', 'operator', 'support', 'aml_specialist', 'compliance'));
