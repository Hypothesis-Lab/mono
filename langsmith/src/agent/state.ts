import { StateSchema } from "@langchain/langgraph";
import { z } from "zod";

/** The first version of the product model intentionally has one field. */
export type Product = {
  description: string;
};

export type ProductValidation = {
  isValid: boolean;
  reason: string;
};

export const DEMO_PRODUCT: Product = {
  description:
    "Дебетовая и кредитная карта в одной: кредитный лимит подключается к дебетовой или зарплатной карте Цифра банка через мобильное приложение.",
};

const productSchema = z.object({ description: z.string() });
const productInputSchema = productSchema.default(() => ({ ...DEMO_PRODUCT }));
const productValidationSchema = z.object({
  isValid: z.boolean(),
  reason: z.string(),
});

/**
 * State is the data that moves through the graph.
 *
 * The Zod defaults are included in the graph input schema so Studio can show
 * the demo product as the initial editable input value.
 */
export const StateAnnotation = new StateSchema({
  product: productInputSchema,
  productValidation: productValidationSchema.default(() => ({
      isValid: false,
      reason: "The product has not been validated yet.",
    })),
});

/** Only product is user input; validation is produced by the graph. */
export const InputState = new StateSchema({
  product: productInputSchema,
});

export const OutputState = new StateSchema({
  product: productSchema,
  productValidation: productValidationSchema,
});
