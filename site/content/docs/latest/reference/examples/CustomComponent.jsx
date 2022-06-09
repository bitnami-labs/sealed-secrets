// Copyright 2021-2022 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

import React, { useEffect, useState } from "react";

//This is a super simple react component to demo how a custom component could look
export default function Test(props) {
  const { param, handleBasicFormParamChange } = props;

  const [value, setValue] = useState(props.value || "");

  useEffect(() => {
    setValue(props.param.value);
  }, [props.param.value]);

  const handleChange = (newValue) => {
    handleBasicFormParamChange(props.param)({
      currentTarget: {
        value: "test",
      },
    });
    setValue(newValue);
  };

  const selectedStyle = {
    backgroundColor: "rgba(0, 140, 255, 0.19)",
  };

  return (
    <>
      <button
        key={`test`}
        type="button"
        onClick={() => handleChange("test")}
        style={"test" === value ? selectedStyle : null}
      >
        Test
      </button>
    </>
  );
}
