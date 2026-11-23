# Startup gate integration

The runtime services have a default-false instance gate but no operator configuration yet. Wire one explicit, default-closed MULTICA_KNOWLEDGE_CONTEXT_ENABLED value through router Config to the router-owned TaskService; collaboration helpers already derive that same instance policy. No mutable global toggle or UI/server-data change. Add self-host forwarding and documented default, and keep generic check environments forcibly closed.

Owned edits: handler Config/constructor, router environment construction, .env.example, selfhost compose, check environment + its shell regression, new real router configuration test. Use the new final test database e7059940, never default/shared databases. The test enables only a test instance and performs no model execution. Actual production enablement remains off until the recorded G2/rollout gates are satisfied.
