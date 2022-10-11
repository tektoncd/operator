# Tekton Operator Release

## What is a Tekton Operator Release

A release constitutes tagged Github release (release.yaml) and a release to operatorhub.io. At present, our goal is to make an operator releases at on a fixed schedule (eg: every 2 weeks) irrespective of the release cycle of other Tekton components (Pipelines, Triggers, Dashboard...). Each release of operator brings the latest available version of each of the components at the time of the release.

# FAQ

1. How often is Tektoncd Operator released?

    First and Fourth Thursday every month.

1. How are Tektoncd Operator releases versioned?
    
    Tekton Operator is versioned following [semver specification](https://semver.org/). The operator version number does not dependent on the version numbers of other Tektoncd components in any way.

    For each release, the minor version will be incremented by 1, and a new `release-vn.n.n` will be made to track the release.

    👉 **Note:** Tektoncd Operator releases before v0.49.0 followed a different versioning scheme. At that time, the `major.minor.patch` in the operator version indicated the Tektoncd Pipelines packaged within that release, and the operators build was represented using an integer prefix (eg: 0.23.0-1). The old versioning scheme started becoming confusing as the operator started supporting Tektoncd components other than Tektoncd Pipelines.

1. What versions of Tektoncd Components will be packaged in a given operator version?

    Each release of Tektoncd Operator will package the latest available released versions of other Tetkoncd components.

    At present, the supported Tektoncd components are

    - [Tektoncd Pipeline](https://github.com/tektoncd/pipeline/releases)
    - [Tektoncd Triggers](https://github.com/tektoncd/triggers/releases)
    - [Tektoncd Dashboard](https://github.com/tektoncd/dashboard/releases)
    - [Tektoncd Results](https://github.com/tektoncd/results/releases)


1. How do we know the version of the Tektoncd components (Pipelines, Triggers, Dashboard ...) that are packaged within a release of Tektoncd Operator?

    At present, we have to refer to the release notes of a Tektoncd Operator release to find out the Tektoncd component versions packaged within that release.

    We are working on making this more user friendly and kubernetes friendly.
    All suggestions, comments, feedback are welcome 🧑‍💻.

1. How are bug fix releases from components made available through operator?

    Consider this scenario: Operator release v0.51.0 is published and this release delivers Tektoncd Pipeline v0.29.0 and Tektoncd Triggers v0.18.0, Tektoncd Dashboard v0.24.0.
    
    After 2 weeks later Operator release v0.52.0 is made with Tektoncd Pipelines v0.29.0, Tektoncd Triggers v0.19.0 and Tektoncd Dashboard v0.24.0.
    
    The next release  after 2 weeks, Operator v0.53.0 comes out with Tektoncd Pipelines v0.30.0, Tektoncd Triggers v0.19.0 and Tektoncd Dashboard v0.25.0.

    Now assume a critical bug is identified in Tektoncd Pipelines v0.29.0 and Tektoncd Pipelines makes a v0.29.1 release. In this case, the operator will make a patch release to all of the previous operator releases which shipped Tetkoncd Pipelines v0.29.0. As a result, the following releases will be made:

    - Tektoncd Operator v0.51.1 : with Tektoncd Pipeline v0.29.1 and Tektoncd Triggers v0.18.0, Tektoncd Dashboard v0.24.0
    - Tektoncd Operator v0.52.1 : with Tektoncd Pipeline v0.29.1 and Tektoncd Triggers v0.18.0, Tektoncd Dashboard v0.24.0

    As v0.51.0 and v0.52.0 are the operator releases which are affected by the bug in Tektoncd Pipelines v0.29.0

    No patch release is needed for Tektoncd Operator v0.53.0 as it ships Tektoncd Pipelines v0.30.0 (not v0.29.0).

1. When (How often) do we do patch releases?

    If a patch release is needed because of a bug fix release in any of the components, then:

    - If the component patch is a critical security patch, we make a operator patch release immediately
    - For all other component patches, we make patch releases weekly on Thursdays
