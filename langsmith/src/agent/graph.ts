import { END, START, StateGraph } from "@langchain/langgraph";
import { InputState, OutputState, StateAnnotation } from "./state.js";

/**
 * The product node is deliberately explicit in the graph so it is easy to see
 * where the product enters the workflow. The product itself is data in state;
 * nodes are the steps that read or transform that data.
 */
const product = (state: typeof StateAnnotation.State) => {
  return { product: state.product };
};

/**
 * A deterministic placeholder for the first product-processing step.
 * Later, this node can call an LLM to extract facts from the description.
 */
const validateProduct = (state: typeof StateAnnotation.State) => {
  const description = state.product.description.trim();

  return {
    productValidation: {
      isValid: description.length > 0,
      reason:
        description.length > 0
          ? "The product has a description."
          : "The product description is required.",
    },
  };
};

// Studio will render this as: productInput -> validateProduct.
const builder = new StateGraph({
  state: StateAnnotation,
  input: InputState,
  output: OutputState,
})
  .addNode("productInput", product)
  .addNode("validateProduct", validateProduct)
  .addEdge(START, "productInput")
  .addEdge("productInput", "validateProduct")
  .addEdge("validateProduct", END);

export const graph = builder.compile();

graph.name = "Product workflow";
