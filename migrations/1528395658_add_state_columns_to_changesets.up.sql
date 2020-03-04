BEGIN;

CREATE TYPE changeset_external_state AS ENUM (
    'open',
    'closed',
    'merged',
    'deleted'
    );

CREATE TYPE changeset_external_review_state AS ENUM (
    'approved',
    'changes_requested',
    'pending',
    'commented',
    'dismissed'
    );

CREATE TYPE changeset_external_check_state AS ENUM (
    'unknown',
    'pending',
    'passed',
    'failed'
    );

ALTER TABLE changesets
    ADD COLUMN external_state changeset_external_state;

ALTER TABLE changesets
    ADD COLUMN external_review_state changeset_external_review_state;

ALTER TABLE changesets
    ADD COLUMN external_check_state changeset_external_check_state;

UPDATE changesets SET external_check_state = 'unknown';

COMMIT;
