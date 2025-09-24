#!/bin/bash

# Import a B2B organization by organization ID
terraform import stytch_b2b_organization.example "project-live-...,organization-live-..."

# Import a B2B organization by slug (automatically resolved to ID)
terraform import stytch_b2b_organization.example "project-live-...,example-slug"

# Import from test environment
terraform import stytch_b2b_organization.test_org "project-test-...,organization-test-..."
