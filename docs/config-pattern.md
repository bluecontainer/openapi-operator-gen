 

Pattern Type	Permanent Change Pattern
Pattern Name	Dynamic Configuration Change with Observer Pattern
Intent	
(Near) Zero-downtime configuration hot-reloading for stateful Real-time (FTRP) workloads on LOCP (Kubernetes) platform 
To enable real-time, persistent application state changes without Pod restarts or deployment rollouts.
Context	Applicable for cloud-native applications in LOCP where downtime or connection resets during config updates are not acceptable.
Motivation	Standard RollingUpdate strategies terminate Pods. The aim is to preserve active sessions while applying new configurations or business logic.
Problem Statement	
Core Problem	Synchronising persistent cluster configuration with the in-memory state of a running container without lifecycle disruption.
Constraints	
Must avoid Pod/ Container restarts

Eliminate manual execution into containers

Challenges	
Managing synchronisation delay / propagation lag and ensuring atomic file reads while swapping configurations.

Solution	


Overview	
This design pattern helps connect fast systems with safe operation. It allows the system to change in a permanent and clear way without needing to restart the container. 

High Level View	
A GitOps-driven workflow where a config file is applied into a Pod. An internal watcher thread listens for a file config change events to trigger updates.

Conceptual background	
If ConfigMap is mounted as a volume (a file inside the container), K8s can update the file without restarting the container. 

When you update a ConfigMap that is mounted as a volume, the Kubelet on the node eventually notices the change and updates the file on the disk. This is not instantaneous change; it can take up to 1-2min (depends on the Kubelet's sync period and cache) for the file inside the container to reflect the next version of the configuration.

Even if the file updates on disk, the binary process of the application must be coded to handle this. Most binary processes read their config file at startup and not refer to the changes to the configurations again.

For this pattern to work without a restart, the application must: 

Watch the file: using a file-watcher library to detect changes
Reload logic: Have a mechanism to re-parse the config and apply the new setting in memory while running.
Key Components for Zero-restart and persistent config updates	
ConfigMap: Durable store of configuration 
Volume Mount: Delivery channel/ a live link which allows K8s to update the filesystem inside container dynamically.
File Watcher: The running binary process must actively observe the filesystem.
Internal Application Logic to apply the new configuration to the running process dynamically.
Workflow & Behaviour	
MR/ PR from App Repo in DxOne triggers the CI/CD pipeline.
Pipeline updates ConfigMap
Kubelet refreshes the volume symlink
The application's file-watcher thread receives an event to reach the new config file.
The application reads the new config file automatically, validates the checksum, parses YAML/JSON, and updates in-memory variables.
The application emits the configuration_reloaded log event containing the new metrics such as reload_duration_ms metric.
Observability platform ingests the metrics; Big Panda correlates against any concurrent alerts. 
Diagrammatic Illustration	
Elektron Contributions > Dynamic Configuration Change > image-2026-3-25_16-26-37.png

Design Principles	
Declarative state
Event Driven Architecture
Separation of concerns (config vs code)
Fail-safe defaults - If the new config is invalid, the application retains the previous state and emits an alert.
Implementation Approach	
Resource Definitions	Deployment volume and volumeMounts projecting a ConfigMap
Configuration	YAML/ JSON files stored within the ConfigMap data
Deployment Strategy	GitOps reconciliation using a CD tool to apply Permanent Change Pattern.
Other Options considered	
API-driven (Ephemeral)	For scenarios where immediate, non-persistent changes are needed, an API end point (from Rundeck) can toggle the state. However, this state is lost when the pod/ container restarts, reverting to the persistent volume configuration value.
Observability	
Logging & Tracing	
Application must log a configuration reloaded event with the new config_hash. Log management tracks these events to provide an audit trial.
Use traces to check if performance is impacted while correlating with the config changes.
Monitoring & Alerting	
Sync delay metric: Calculate the delta between the ConfigMap Update Timestamp and the App Reload Timestamp.
Anomaly Detection: Use Anomaly monitor on the reload latency. If sync takes longer than 120s, an alert can be triggered.
Event Correlation (BigPanda)	
Alert Aggregation
Root Cause Analysis
Anti-pattern warning	
K8s Secretes	
This pattern is not applicable for Secrets rotation, since K8s Secretes mounted as volumes require additional security considerations such as RBAC, encryptions, and more comprehensive audit-logging.

Trade-offs	
Advantages	
Zero downtime
Persistent across restarts
Auditability in DxOne
Disadvantages	
Complexity in application code (requires an implementation of watcher thread and reload logic while running)
Kubelet Sync Latency: Changes are not instantaneous. Depending on ConfigMapAndSecretChangeDetectionStrategy setting in the Kubelet, it can take up to 1-2 min for the file change to reflect inside the container. 
Technical Boundaries/ Limitations	
Namespace Isolation: ConfigMaps are namespaced resources. A pod in one namesapce cannot natively mount or observe a ConfigMap located in another namespace. This requires duplicating configurations across namespaces.
Scope of Clusters: This pattern is local to a single cluster. For a global service running across multiple regions, CI/CD pipelines must ensure the ConfigMap is updated across all clusters individually. There is no native cross-cluster ConfigMap mirroring.
If ConfigMap is mounted using the subPath property (to mount a single file into an existing directory), it will not update dynamically. K8s only refreshes volumes mounted as entire directories.
Size Constraints: ConfigMaps have a hard limit of 1MB. For larger config files, PersistentVolume needs to be used.
Glossary	
To be completed after the initial review.	
 