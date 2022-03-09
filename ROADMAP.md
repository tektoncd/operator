# Roadmap

## Driving adoption

- Add more docs, and have it publish as part of the tekton.dev/docs
  website.
- Highlight the benefit of using the operator, and mainly what pain it
  solves

An acceptance criteria for this would be to use the operator in our
own CI (aka dogfooding).

## New Component integration

As of today, the operator is capable of installing Pipeline, Triggers
and the dasbhoard. We may want to support shipping more components

### Components

- New "graduated" components

### Experimental projects

- custom tasks
- other projects

## Add Catalog ClusterTask to all targets

Today, we are shipping ClusterTask only for the OpenShift target. We
should aim towards shipping this for all targets (k8s, …)

## Tekton CLI integration

User should be able to get `tkn` and install, upgrade and manage the
operator lifecycle directly from it. *This should help adoption as well*.

## Support rollback

In case of a failed upgrade, it should be possible to roll-back into
the previous known good state.

## More targets

We are currently targeting and releasing only two target:
- Vanilla k8s
- OpenShift

We should aim to support more, starting with GKE. GKE is a easy target
as we could use this in dogfooding. An idea of what could be specific
for GKE is around the ingress configuration, …

## More tests, more confidence

The operator codebase integrates all tektoncd component into one
place, it is a critical piece and need to be heavily tested so that we
feel confident to release it.

- Upgrade tests
- Running component tests on top of an operator installation
- etc…

## Releases

- Automated relases
- Automated publication on OperatorHub
- More often (monthly)
- Better management of the payloads (releases yamls)
  Today, we do some manual changes in order to release, we should aim
  towards taking the upstream releases yamls as is and *fix* anything
  programmatically (if need be).

## Metrics

Enable metrics on the operator, to be able to gather information on
the health of managed components from the operator.
