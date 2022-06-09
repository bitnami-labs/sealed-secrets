// Copyright 2021-2022 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

import React from "react";

const CustomAppView = (props) => {
  const { handleRedirect, handleDelete, handleRollback, app } = props;
  const { cluster, namespace } = app.installedPackageRef.context;
  const valuesApplied = JSON.parse(props.app.valuesApplied);
  const { upgrade, list } = props.urls.app.apps;

  const onUpgrade = () => handleRedirect(upgrade(app.installedPackageRef));

  const onDelete = async () => {
    if (confirm("Are you sure you want to delete?")) {
      const deleted = await handleDelete();
      if (deleted) {
        handleRedirect(list(app.installedPackageRef));
      }
    }
  };

  const onRollback = async () => {
    const rolledBack = await handleRollback();
    if (rolledBack) {
      alert("Rolled back!");
    }
  };

  const { latestVersion, currentVersion } = app;
  const latestApp = latestVersion?.appVersion;
  const latestPkg = latestVersion?.pkgVersion;
  const currentApp = currentVersion?.appVersion;
  const currentPkg = currentVersion?.pkgVersion;
  const appOutOfDate = latestApp && currentApp && currentApp !== latestApp;
  const pkgOutOfDate = latestPkg && currentPkg && latestPkg !== currentPkg;

  return (
    <div>
      <h1>{props.app.name}</h1>
      <p>{props.appDetails.shortDescription}</p>
      <button onClick={onDelete}>Delete</button>
      <button onClick={onUpgrade}>Upgrade</button>
      <button onClick={onRollback}>Rollback</button>
      {appOutOfDate && (
        <button onClick={onUpgrade}>Update App to v{latestApp}</button>
      )}
      {pkgOutOfDate && (
        <button onClick={onUpgrade}>Update package to v{latestPkg}</button>
      )}
      <h3>{namespace}</h3>
      <h3>{cluster}</h3>
      {!appOutOfDate && !pkgOutOfDate && "Everything is up to date!"}
      {Object.entries(valuesApplied).map(([key, val]) => (
        <p key={key}>
          <strong>{key}</strong>
          {": "}
          {JSON.stringify(val)}
        </p>
      ))}
    </div>
  );
};

export default CustomAppView;
