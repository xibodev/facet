import React from "react";
import {CalculateMetadataFunction, Composition} from "remotion";

import {Explainer} from "./Explainer";
import {
  DEFAULT_COMPOSITION,
  ExplainerProps,
  validateComposition,
} from "./contract";

const defaultProps: ExplainerProps = {
  cuts: [
    {
      type: "text_card",
      text: "Facet Explainer",
      in_seconds: 0,
      out_seconds: 2,
    },
  ],
};

const calculateMetadata: CalculateMetadataFunction<ExplainerProps> = ({
  props,
}) => {
  const composition = validateComposition(props);
  return {
    width: composition.width,
    height: composition.height,
    fps: composition.fps,
    durationInFrames: composition.durationInFrames,
    props: composition,
  };
};

export const Root: React.FC = () => (
  <Composition
    id="Explainer"
    component={Explainer}
    width={DEFAULT_COMPOSITION.width}
    height={DEFAULT_COMPOSITION.height}
    fps={DEFAULT_COMPOSITION.fps}
    durationInFrames={DEFAULT_COMPOSITION.fps * 2}
    defaultProps={defaultProps}
    calculateMetadata={calculateMetadata}
  />
);
