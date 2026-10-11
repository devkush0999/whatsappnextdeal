# Reusable CRM messaging platform roadmap

Planning baseline: approximately 300 outbound customer messages per day. If this means 300 customers with multiple exchanges, capacity must include every incoming/outgoing message, media transfer, status update and automation action. This is a design target, not a measured benchmark or a WhatsApp account allowance.

## Product boundary

Extend the existing Evolution Go backend with modular Go services for conversations, permissions, delivery jobs and automation. Keep WhatsApp transport behind a Go provider interface. CRM, ERP, billing and other apps integrate through the same API. Their business records remain in their own applications and are referenced by external IDs.

The existing gateway provides instance management, QR pairing, send APIs and event integrations. Local text and PDF send tests returned success; a complete shared inbox and general-purpose workflow engine are not yet implemented. The Go OTP module now provides project-scoped credentials, atomic verification, a durable delivery queue, and an English admin console; see `docs/OTP_SERVICE.md`. This OTP isolation does not resolve every existing gateway authorization issue. Provider-dependent endpoints must not be presented as guaranteed capabilities.

For commercial automated messaging, prioritize an official WhatsApp Business Platform adapter. The current whatsmeow integration uses the linked-device protocol. Self-hosting and a low daily volume do not guarantee account safety, protocol stability or an approved automation channel.

## Priority definitions

- P0: required before using real customer data or dependable automation.
- P1: first useful CRM product release.
- P2: growth, advanced workflows and additional channels.

## Feature backlog

| # | Feature | Priority | Behavior / acceptance requirement |
|---|---|---|---|
| 1 | Organizations and projects | P0 | Isolate customers, instances, messages and integrations by tenant and project. |
| 2 | User authentication | P0 | Human users sign in without receiving the gateway administrator key. |
| 3 | Roles and permissions | P0 | Owner, administrator, supervisor, agent and integration-service roles. |
| 4 | Scoped API credentials | P0 | Per-project scopes, expiration, rotation, revocation and audit metadata. |
| 5 | Team and inbox membership | P1 | Restrict which conversations each team can access. |
| 6 | Audit trail | P0 | Record who viewed/exported sensitive data or changed integrations and settings. |
| 7 | Multiple WhatsApp connections | P1 | Map each number to an explicit tenant, project and inbox. |
| 8 | Live onboarding | P0 | Refresh QR codes, show connection stages, expiration and actionable failures. |
| 9 | Session ownership | P0 | Prevent duplicate starts and competing processes using the same session. |
| 10 | Session lifecycle | P0 | Separate reconnect, disconnect, revoke and delete operations. |
| 11 | Connection alerts | P0 | Notify operators when a number cannot send or needs relinking. |
| 12 | Provider capability registry | P1 | Advertise which message types and actions each provider actually supports. |
| 13 | Conversation list | P1 | Search, unread counts, last message, assignment and open/resolved state. |
| 14 | Live chat timeline | P1 | Persist and display incoming/outgoing messages with cursor pagination. |
| 15 | Message composer | P1 | Text, image, document, supported audio/video and attachment previews. |
| 16 | Accurate delivery states | P0 | Distinguish queued, submitted, provider-accepted, delivered, read and failed. |
| 17 | Reply context and reactions | P1 | Display quoted messages and supported reactions without promising universal compatibility. |
| 18 | Agent assignment | P1 | Manual assignment, reassignment and unassigned queue. |
| 19 | Internal notes and mentions | P1 | Clearly separate staff-only notes from customer-visible messages. |
| 20 | Quick replies | P1 | Searchable reusable text with validated customer variables. |
| 21 | Collision prevention | P1 | Show when another agent is replying; prevent duplicate automated/human replies. |
| 22 | Conversation lifecycle | P1 | Open, pending, snoozed, resolved and reopened states. |
| 23 | Contact directory | P1 | Normalize phone numbers, associate provider identities and deduplicate within a tenant. |
| 24 | Customer context panel | P1 | Read linked lead, order, invoice and ticket information with permissions. |
| 25 | Tags and custom fields | P1 | Customer type, language, region, interest and business-specific fields. |
| 26 | Consent records | P0 | Store permission source, timestamp, purpose and withdrawal. |
| 27 | Suppression list | P0 | Apply opt-outs across jobs, campaigns and automated responses where relevant. |
| 28 | Contact import | P1 | CSV import with validation, deduplication and consent provenance. |
| 29 | Contact export controls | P1 | Restrict exports and audit them. |
| 30 | API versioning | P0 | Stable contracts independent of the underlying gateway/provider. |
| 31 | Integration SDKs | P1 | Go OTP client is implemented; expand typed clients for other APIs/languages as needed. |
| 32 | External record mapping | P0 | Link messages to project-scoped customer/order/invoice/ticket IDs. |
| 33 | Signed outgoing webhooks | P0 | HMAC, timestamp, event ID, signature rotation and replay protection. |
| 34 | Webhook delivery dashboard | P1 | Inspect redacted failures and replay authorized events. |
| 35 | Incoming event validation | P0 | Verify provider signatures/tokens and enforce payload limits. |
| 36 | Integration test environment | P1 | Synthetic events and dry-run sends without contacting customers. |
| 37 | Durable outbound queue | P0 | Persist intent before acknowledging a send request. |
| 38 | Idempotent send API | P0 | Repeated requests with the same project-scoped key create one logical job. |
| 39 | Incoming deduplication | P0 | Process repeated provider events without duplicate messages or workflows. |
| 40 | Retry policy | P0 | Backoff, bounded attempts and transient/permanent failure classification. |
| 41 | Ambiguous-send reconciliation | P0 | Handle timeouts after provider acceptance without blind duplicate sends. |
| 42 | Failed-job queue | P0 | Retain exhausted jobs for inspection and controlled retry. |
| 43 | Per-number concurrency control | P0 | Preserve appropriate ordering and honor provider limits. |
| 44 | Message priorities | P1 | Prioritize active support and transactional updates over campaigns. |
| 45 | Scheduling and cancellation | P1 | Timezone-aware scheduled sends and cancellation before dispatch. |
| 46 | Expiring jobs | P0 | Do not send obsolete reminders after a long outage. |
| 47 | Global/project pause controls | P0 | Stop queued automation quickly during an incident. |
| 48 | Message template library | P1 | Variables, previews, validation, language and versioning. |
| 49 | Official template synchronization | P0 for Cloud API | Track provider approval/status; do not confuse local snippets with approved templates. |
| 50 | Service-window enforcement | P0 for Cloud API | Select permitted free-form replies or approved templates based on conversation state. |
| 51 | Campaign audience builder | P2 | Segment opted-in contacts and show estimated audience before starting. |
| 52 | Campaign approvals and preview | P2 | Review sample rendered messages and exclude suppressed contacts. |
| 53 | Frequency caps and quiet hours | P1 | Apply recipient/project rules in the recipient's configured timezone. |
| 54 | Campaign pause/resume | P2 | Resume only remaining eligible jobs without duplicate sends. |
| 55 | Trigger-condition-action rules | P1 | Begin with a simple form-based rule editor rather than a visual workflow canvas. |
| 56 | Welcome and away replies | P1 | Respect business hours and avoid replying to the bot's own messages. |
| 57 | Keyword routing | P1 | Route sales, support and billing intents to the correct queue. |
| 58 | Follow-up automation | P1 | Stop when the customer responds, opts out or the business record changes. |
| 59 | Workflow execution history | P0 | Explain each trigger, decision, wait, send and cancellation. |
| 60 | Business-time waits and SLA escalation | P1 | Escalate unassigned or overdue conversations to a supervisor. |
| 61 | Loop and execution limits | P0 | Cap steps, recurrence and messages per workflow/customer. |
| 62 | Visual workflow builder | P2 | Version, validate, test and publish flows after core execution is reliable. |
| 63 | CRM lead integration | P1 | Create/update leads from inbound chats and associate an owner. |
| 64 | Order and dispatch updates | P1 | Send event-driven confirmations using authoritative order state. |
| 65 | Invoice and receipt delivery | P1 | Send a generated PDF or an authorized expiring link. |
| 66 | Payment notifications | P1 | Confirm payment only from verified payment-provider events. |
| 67 | Appointment reminders | P1 | Confirm, reschedule, cancel and invalidate outdated reminders. |
| 68 | Ticket integration | P1 | Create support tickets and send status updates. |
| 69 | Field-service communication | P2 | Link booking, technician assignment and completion events. |
| 70 | HR/internal notifications | P2 | Use access-controlled links instead of sensitive document content where appropriate. |
| 71 | Agent-assisted AI drafts | P2 | Suggest replies with agent approval before enabling autonomous responses. |
| 72 | Knowledge-base answers | P2 | Restrict retrieval to the correct tenant and current approved documents. |
| 73 | Conversation summaries | P2 | Produce handoff summaries with source references where feasible. |
| 74 | Intent and language detection | P2 | Assist routing while allowing staff correction. |
| 75 | AI human handoff | P1 if AI enabled | Pause automation on takeover and route uncertainty to a person. |
| 76 | AI tool permissions | P0 if AI enabled | No unapproved payments, refunds, order changes or cross-tenant data access. |
| 77 | AI data controls | P0 if AI enabled | Disclose external processing, minimize payloads and configure retention. |
| 78 | Tenant authorization fixes | P0 | Enforce ownership in every handler/query, including settings and message status. |
| 79 | Secret-safe events/logs | P0 | Remove API keys, pairing secrets and session tokens from events and routine logs. |
| 80 | Private media storage | P0 | Private objects, short-lived signed links and tenant authorization. |
| 81 | URL-fetch protections | P0 | Restrict destination addresses/redirects, sizes, content types and timeouts. |
| 82 | Integration destination controls | P0 | Require explicit trusted webhook, proxy and converter configuration. |
| 83 | Data retention and deletion | P1 | Separate message, attachment, audit, session and backup retention policies. |
| 84 | Backup and restore | P0 | Encrypt backups and regularly restore application and session data in a safe environment. |
| 85 | Network and deployment hardening | P0 | HTTPS, private database ports, loopback development binding and least-privilege service accounts. |
| 86 | Delivery and queue metrics | P0 | Track failures, oldest queued job, retries, provider latency and disconnect duration. |
| 87 | Agent/SLA reporting | P1 | First-response time, resolution time and queue workload. |
| 88 | Automation outcomes | P1 | Measure completed flows, suppressed sends, handoffs and failures. |
| 89 | Usage and budget controls | P1 | Separate message counts, unique recipients, provider costs and AI usage. |
| 90 | Mobile agent inbox | P2 | React Native inbox, push notifications, attachments and reconnect recovery. |
| 91 | Responsive accessible web inbox | P1 | Keyboard navigation, readable contrast, responsive layout and localization. |
| 92 | CI/CD and rollback | P0 | Versioned images, migrations, secrets per environment and tested rollback procedures. |
| 93 | Boundary regression tests | P0 | Tenant isolation, webhook signatures, opt-outs and secret redaction. |
| 94 | Reliability tests | P0 | Duplicate events, restart recovery, delayed/out-of-order receipts and uncertain send outcomes. |
| 95 | Load and soak tests | P0 | Simulate representative text/media peaks without flooding real customer accounts. |
| 96 | Additional channels | P2 | Add email/SMS adapters with their own consent, capabilities and delivery rules. |

## Recommended implementation

- React web inbox first; add a React Native agent app after the workflow is stable.
- Go + Gin with Handler -> Service -> Repository modules inside the existing backend.
- GORM + PostgreSQL for tenants, projects, contacts, conversations, messages, jobs and audit data.
- PostgreSQL-backed jobs written atomically with business records, claimed by Go workers. The OTP queue already follows this design. Add a separate broker only when measured requirements justify it.
- Keep WhatsApp sessions and business services in the Go process behind provider interfaces. Capability checks must prevent unsupported cross-provider behavior.
- The main CRM remains the source of truth for orders, invoices, payments and leads. Messaging stores references, safe display snapshots and communication state.
- Keep administrator and provider credentials on the backend, never inside a mobile app or browser integration SDK.

Core tables: Tenant, Project, User, Membership, ChannelConnection, Contact, ConsentRecord, Conversation, ConversationAssignment, Message, MessageReceipt, Attachment, Template, OutboxEvent, DeliveryJob, WebhookSubscription, WebhookDelivery, AutomationDefinition, AutomationRun, AuditEvent.

Use tenant-scoped foreign keys and uniqueness constraints. Index conversation timelines by tenant/conversation/time and delivery jobs by state/scheduled time. Keep provider message IDs, application message IDs and client idempotency keys distinct.

## Initial API surface

- POST /v1/messages - accepts a project-scoped idempotency key, creates a durable job, returns 202 and an application message ID.
- GET /v1/messages/:id - returns current state, provider IDs and sanitized failure information.
- GET /v1/conversations and GET /v1/conversations/:id/messages - cursor-paginated inbox and timeline.
- POST /v1/conversations/:id/assignments - permission-checked assignment.
- POST /v1/contacts and PUT /v1/contacts/:id/consent - validated contact and permission updates.
- POST /v1/automations and POST /v1/automations/:id/test - versioned rules and dry runs.
- POST /v1/webhook-subscriptions - explicitly configured, signed event forwarding.

## Example workflows

1. CRM lead created -> check consent -> select allowed greeting -> enqueue -> assign owner -> cancel follow-up if a reply arrives.
2. Invoice finalized -> render document -> send approved notification/document as allowed -> update delivery state -> cancel pending reminder after verified payment.
3. Customer sends support request -> store message -> route to team -> send allowed acknowledgement -> escalate if SLA expires -> pause bot when an agent takes over.
4. Appointment booked -> schedule reminder -> recheck booking state at send time -> cancel or replace jobs if rescheduled.

## Delivery sequence

1. Secure foundation: isolation, log/event redaction, connection lifecycle, private storage, durable jobs and message persistence. Gate: a restart does not lose jobs or duplicate sends, and cross-project access is denied.
2. Usable CRM inbox: contacts, history, live updates, composer, assignment, notes, quick replies and customer context. Gate: a two-way conversation is visible after refresh and reconnect.
3. Business automation: triggers, scheduling, approved templates where applicable, consent enforcement, cancel conditions and execution history. Gate: retries, opt-outs and state changes do not cause inappropriate sends.
4. Expansion: campaign tooling, AI assistance, mobile inbox, advanced analytics and additional channels.

## WhatsApp policy references

Checked 11 October 2026. Recipient opt-in and opt-out handling are required by the business policy. On the official Business Platform, free-form replies are permitted within the customer service window; outside it, approved templates are required. Automated replies need clear escalation paths. These Platform rules do not turn an unofficial linked-device adapter into an approved provider.

- [WhatsApp Business Messaging Policy](https://business.whatsapp.com/policy)
- [WhatsApp Messaging Guidelines](https://www.whatsapp.com/legal/messaging-guidelines)

There is no universal safe daily-message count for the current gateway. Do not implement account rotation, random-delay promises or retries intended to bypass provider restrictions. Apply actual provider limits and permission rules.
