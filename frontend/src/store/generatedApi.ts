import { emptySplitApi as api } from "./emptyApi";
const injectedRtkApi = api.injectEndpoints({
  endpoints: (build) => ({
    registerUser: build.mutation<RegisterUserApiResponse, RegisterUserApiArg>({
      query: (queryArg) => ({
        url: `/api/v1/auth/register`,
        method: "POST",
        body: queryArg.registerUserRequest,
      }),
    }),
    loginUser: build.mutation<LoginUserApiResponse, LoginUserApiArg>({
      query: (queryArg) => ({
        url: `/api/v1/auth/login`,
        method: "POST",
        body: queryArg.loginUserRequest,
      }),
    }),
    getYandexDirectConnection: build.query<
      GetYandexDirectConnectionApiResponse,
      GetYandexDirectConnectionApiArg
    >({
      query: () => ({ url: `/api/v1/yandex-direct/connection` }),
    }),
    connectYandexDirect: build.mutation<
      ConnectYandexDirectApiResponse,
      ConnectYandexDirectApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/yandex-direct/connection`,
        method: "POST",
        body: queryArg.yandexDirectConnectRequest,
      }),
    }),
    disconnectYandexDirect: build.mutation<
      DisconnectYandexDirectApiResponse,
      DisconnectYandexDirectApiArg
    >({
      query: () => ({
        url: `/api/v1/yandex-direct/connection`,
        method: "DELETE",
      }),
    }),
    getYandexDirectAuthorizationUrl: build.query<
      GetYandexDirectAuthorizationUrlApiResponse,
      GetYandexDirectAuthorizationUrlApiArg
    >({
      query: () => ({ url: `/api/v1/yandex-direct/authorization-url` }),
    }),
    runProductFeedbackSurvey: build.mutation<
      RunProductFeedbackSurveyApiResponse,
      RunProductFeedbackSurveyApiArg
    >({
      query: () => ({
        url: `/api/v1/testing/product-feedback-survey`,
        method: "POST",
      }),
    }),
    listProductFeedbackSurveyResults: build.query<
      ListProductFeedbackSurveyResultsApiResponse,
      ListProductFeedbackSurveyResultsApiArg
    >({
      query: () => ({ url: `/api/v1/testing/product-feedback-survey/results` }),
    }),
    listHypotheses: build.query<
      ListHypothesesApiResponse,
      ListHypothesesApiArg
    >({
      query: () => ({ url: `/api/v1/hypothesis` }),
    }),
    createHypothesis: build.mutation<
      CreateHypothesisApiResponse,
      CreateHypothesisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis`,
        method: "POST",
        body: queryArg.createHypothesisRequest,
      }),
    }),
    getHypothesis: build.query<GetHypothesisApiResponse, GetHypothesisApiArg>({
      query: (queryArg) => ({ url: `/api/v1/hypothesis/${queryArg.id}` }),
    }),
    updateHypothesis: build.mutation<
      UpdateHypothesisApiResponse,
      UpdateHypothesisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}`,
        method: "PATCH",
        body: queryArg.updateHypothesisRequest,
      }),
    }),
    deleteHypothesis: build.mutation<
      DeleteHypothesisApiResponse,
      DeleteHypothesisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}`,
        method: "DELETE",
      }),
    }),
    launchHypothesis: build.mutation<
      LaunchHypothesisApiResponse,
      LaunchHypothesisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}/launch`,
        method: "POST",
      }),
    }),
    getHypothesisLaunchStatus: build.query<
      GetHypothesisLaunchStatusApiResponse,
      GetHypothesisLaunchStatusApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}/launch-status`,
      }),
    }),
    stopHypothesis: build.mutation<
      StopHypothesisApiResponse,
      StopHypothesisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}/stop`,
        method: "POST",
      }),
    }),
    getHypothesisProduct: build.query<
      GetHypothesisProductApiResponse,
      GetHypothesisProductApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}/product`,
      }),
    }),
    createProduct: build.mutation<
      CreateProductApiResponse,
      CreateProductApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product`,
        method: "POST",
        body: queryArg.createProductRequest,
      }),
    }),
    updateProduct: build.mutation<
      UpdateProductApiResponse,
      UpdateProductApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}`,
        method: "PATCH",
        body: queryArg.updateProductRequest,
      }),
    }),
    deleteProduct: build.mutation<
      DeleteProductApiResponse,
      DeleteProductApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}`,
        method: "DELETE",
      }),
    }),
    getProductAnalysis: build.query<
      GetProductAnalysisApiResponse,
      GetProductAnalysisApiArg
    >({
      query: (queryArg) => ({ url: `/api/v1/product/${queryArg.id}/analysis` }),
    }),
    startProductAnalysis: build.mutation<
      StartProductAnalysisApiResponse,
      StartProductAnalysisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/analysis`,
        method: "POST",
      }),
    }),
    getProductAnalysisStatus: build.query<
      GetProductAnalysisStatusApiResponse,
      GetProductAnalysisStatusApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/analysis/status`,
      }),
    }),
    saveProductAnalysisReview: build.mutation<
      SaveProductAnalysisReviewApiResponse,
      SaveProductAnalysisReviewApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/analysis/review`,
        method: "PUT",
        body: queryArg.productAnalysisReviewRequest,
      }),
    }),
    continueProductAnalysis: build.mutation<
      ContinueProductAnalysisApiResponse,
      ContinueProductAnalysisApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/analysis/continue`,
        method: "POST",
      }),
    }),
    listProductFiles: build.query<
      ListProductFilesApiResponse,
      ListProductFilesApiArg
    >({
      query: (queryArg) => ({ url: `/api/v1/hypothesis/${queryArg.id}/files` }),
    }),
    uploadProductFile: build.mutation<
      UploadProductFileApiResponse,
      UploadProductFileApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/hypothesis/${queryArg.id}/files`,
        method: "POST",
        body: queryArg.body,
      }),
    }),
    deleteProductFile: build.mutation<
      DeleteProductFileApiResponse,
      DeleteProductFileApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/files/${queryArg.id}`,
        method: "DELETE",
      }),
    }),
    queryPersonas: build.mutation<
      QueryPersonasApiResponse,
      QueryPersonasApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/personas/query`,
        method: "POST",
        body: queryArg.personaQueryRequest,
      }),
    }),
  }),
  overrideExisting: false,
});
export { injectedRtkApi as api };
export type RegisterUserApiResponse =
  /** status 201 registered and logged in */ AuthTokenResponse;
export type RegisterUserApiArg = {
  registerUserRequest: RegisterUserRequest;
};
export type LoginUserApiResponse =
  /** status 200 authenticated */ AuthTokenResponse;
export type LoginUserApiArg = {
  loginUserRequest: LoginUserRequest;
};
export type GetYandexDirectConnectionApiResponse =
  /** status 200 Yandex Direct connection status */ YandexDirectConnectionResponse;
export type GetYandexDirectConnectionApiArg = void;
export type ConnectYandexDirectApiResponse =
  /** status 200 Yandex Direct connection status */ YandexDirectConnectionResponse;
export type ConnectYandexDirectApiArg = {
  yandexDirectConnectRequest: YandexDirectConnectRequest;
};
export type DisconnectYandexDirectApiResponse = unknown;
export type DisconnectYandexDirectApiArg = void;
export type GetYandexDirectAuthorizationUrlApiResponse =
  /** status 200 Yandex authorization URL */ YandexDirectAuthorizationUrlResponse;
export type GetYandexDirectAuthorizationUrlApiArg = void;
export type RunProductFeedbackSurveyApiResponse =
  /** status 200 product feedback survey batch completed */ ProductFeedbackSurveyRunResponse;
export type RunProductFeedbackSurveyApiArg = void;
export type ListProductFeedbackSurveyResultsApiResponse =
  /** status 200 stored product feedback survey results */ ProductFeedbackSurveyResultsResponse;
export type ListProductFeedbackSurveyResultsApiArg = void;
export type ListHypothesesApiResponse =
  /** status 200 list of hypotheses */ HypothesisResponse[];
export type ListHypothesesApiArg = void;
export type CreateHypothesisApiResponse =
  /** status 201 created hypothesis */ HypothesisResponse;
export type CreateHypothesisApiArg = {
  createHypothesisRequest: CreateHypothesisRequest;
};
export type GetHypothesisApiResponse =
  /** status 200 hypothesis */ HypothesisResponse;
export type GetHypothesisApiArg = {
  id: string;
};
export type UpdateHypothesisApiResponse =
  /** status 200 updated hypothesis */ HypothesisResponse;
export type UpdateHypothesisApiArg = {
  id: string;
  updateHypothesisRequest: UpdateHypothesisRequest;
};
export type DeleteHypothesisApiResponse = unknown;
export type DeleteHypothesisApiArg = {
  id: string;
};
export type LaunchHypothesisApiResponse =
  /** status 202 hypothesis launch queued */ HypothesisLaunchResponse;
export type LaunchHypothesisApiArg = {
  id: string;
};
export type GetHypothesisLaunchStatusApiResponse =
  /** status 200 hypothesis launch status */ HypothesisLaunchStatusResponse;
export type GetHypothesisLaunchStatusApiArg = {
  id: string;
};
export type StopHypothesisApiResponse =
  | /** status 200 hypothesis is already stopped */ HypothesisResponse
  | /** status 202 hypothesis stop queued */ HypothesisStopResponse;
export type StopHypothesisApiArg = {
  id: string;
};
export type GetHypothesisProductApiResponse =
  /** status 200 product attached to the hypothesis */ ProductResponse;
export type GetHypothesisProductApiArg = {
  id: string;
};
export type CreateProductApiResponse =
  /** status 201 created product */ ProductResponse;
export type CreateProductApiArg = {
  createProductRequest: CreateProductRequest;
};
export type UpdateProductApiResponse =
  /** status 200 updated product */ ProductResponse;
export type UpdateProductApiArg = {
  id: string;
  updateProductRequest: UpdateProductRequest;
};
export type DeleteProductApiResponse = unknown;
export type DeleteProductApiArg = {
  id: string;
};
export type GetProductAnalysisApiResponse =
  /** status 200 latest analysis */ ProductAnalysisResponse;
export type GetProductAnalysisApiArg = {
  id: string;
};
export type StartProductAnalysisApiResponse =
  /** status 201 analysis queued */ ProductAnalysisResponse;
export type StartProductAnalysisApiArg = {
  id: string;
};
export type GetProductAnalysisStatusApiResponse =
  /** status 200 latest analysis status */ ProductAnalysisStatusResponse;
export type GetProductAnalysisStatusApiArg = {
  id: string;
};
export type SaveProductAnalysisReviewApiResponse =
  /** status 200 saved stage review */ ProductAnalysisReviewResponse;
export type SaveProductAnalysisReviewApiArg = {
  id: string;
  productAnalysisReviewRequest: ProductAnalysisReviewRequest;
};
export type ContinueProductAnalysisApiResponse =
  /** status 200 analysis resumed */ ProductAnalysisResponse;
export type ContinueProductAnalysisApiArg = {
  id: string;
};
export type ListProductFilesApiResponse =
  /** status 200 list of product files */ ProductFileResponse[];
export type ListProductFilesApiArg = {
  id: string;
};
export type UploadProductFileApiResponse =
  /** status 201 uploaded product file */ ProductFileResponse;
export type UploadProductFileApiArg = {
  id: string;
  body: {
    /** The file to upload. Only .docx, .doc, .pdf, .txt, and .md files are accepted. */
    file: Blob;
    type: "product" | "brandbook" | "reference" | "other";
  };
};
export type DeleteProductFileApiResponse = unknown;
export type DeleteProductFileApiArg = {
  id: string;
};
export type QueryPersonasApiResponse =
  /** status 200 matching personas */ PersonaQueryResponse;
export type QueryPersonasApiArg = {
  personaQueryRequest: PersonaQueryRequest;
};
export type AuthTokenResponse = {
  token: string;
};
export type ErrDetail = {
  code: number;
  description: string;
  error_code?: string;
};
export type ErrBody = {
  error: ErrDetail;
};
export type RegisterUserRequest = {
  email: string;
  password: string;
};
export type LoginUserRequest = {
  email: string;
  password: string;
};
export type YandexDirectConnectionResponse = {
  status: "connected" | "disconnected" | "revoked";
  connected_at: string | null;
  disconnected_at: string | null;
  token_expires_at: string | null;
};
export type YandexDirectConnectRequest = {
  code: string;
};
export type YandexDirectAuthorizationUrlResponse = {
  authorization_url: string;
};
export type ProductFeedbackSurveyQuestion = {
  type: string;
  instructions: string;
  criteria?: any;
};
export type ProductFeedbackSurveyQuestionnaire = {
  [key: string]: ProductFeedbackSurveyQuestion;
};
export type ProductFeedbackSurveyRunResponse = {
  count: number;
  questionnaire: ProductFeedbackSurveyQuestionnaire;
};
export type ProductFeedbackSurveyPersona = {
  id: string;
  version?: string;
  source?: string;
  display_name: string;
  dimensions?: {
    [key: string]: string;
  };
};
export type ProductFeedbackSurveyResultResponse = {
  persona_id: string;
  persona_name: string;
  persona: ProductFeedbackSurveyPersona;
  response: any;
};
export type ProductFeedbackSurveyResultsResponse = {
  questionnaire: ProductFeedbackSurveyQuestionnaire;
  results: ProductFeedbackSurveyResultResponse[];
};
export type HypothesisResponse = {
  id: string;
  description: string;
  status: string;
  created_at: string;
};
export type CreateHypothesisRequest = {
  description: string;
};
export type UpdateHypothesisRequest = {
  description?: string | null;
};
export type HypothesisLaunchResponse = {
  workflow_id: string;
  status: "queued";
};
export type HypothesisLaunchStatusResponse = {
  workflow_id: string;
  status:
    | "queued"
    | "running"
    | "completed"
    | "failed"
    | "canceled"
    | "timed_out"
    | "terminated";
};
export type HypothesisStopResponse = {
  workflow_id: string;
  status: "queued";
};
export type ProductCategory =
  | "electronics"
  | "clothing"
  | "food"
  | "furniture"
  | "beauty"
  | "sports"
  | "toys"
  | "books"
  | "automotive"
  | "health"
  | "home"
  | "office"
  | "garden"
  | "pets"
  | "music"
  | "banking_card"
  | "other";
export type ProductPrice = {
  status: "unknown" | "known";
  description: string;
};
export type ProductResponse = {
  id: string;
  name: string;
  description: string;
  category: ProductCategory;
  custom_category?: string | null;
  brand: string;
  country: string;
  town?: string | null;
  experiment_goal: string;
  goal_origin: "demo_assumption" | "user_provided";
  target_action: string;
  primary_metric: string;
  metric_direction: "increase" | "decrease";
  metric_numerator: string;
  metric_denominator: string;
  success_criterion?: string | null;
  price: ProductPrice;
  allowed_channels: string[];
  forbidden_claims: string[];
  known_constraints: string[];
  unknowns: string[];
  keywords: string[];
  link?: string | null;
  hypothesis_id: string;
};
export type CreateProductRequest = {
  name: string;
  description: string;
  category: ProductCategory;
  custom_category?: string;
  brand: string;
  country: string;
  town?: string | null;
  experiment_goal: string;
  goal_origin: "demo_assumption" | "user_provided";
  target_action: string;
  primary_metric: string;
  metric_direction: "increase" | "decrease";
  metric_numerator: string;
  metric_denominator: string;
  success_criterion?: string | null;
  price: ProductPrice;
  allowed_channels: string[];
  forbidden_claims: string[];
  known_constraints: string[];
  unknowns: string[];
  keywords?: string[];
  link?: string | null;
  hypothesis_id: string;
};
export type UpdateProductRequest = {
  name?: string | null;
  description?: string | null;
  category?: ProductCategory;
  custom_category?: string;
  brand?: string;
  country?: string;
  town?: string | null;
  experiment_goal?: string;
  goal_origin?: "demo_assumption" | "user_provided";
  target_action?: string;
  primary_metric?: string;
  metric_direction?: "increase" | "decrease";
  metric_numerator?: string;
  metric_denominator?: string;
  success_criterion?: string | null;
  price?: ProductPrice;
  allowed_channels?: string[];
  forbidden_claims?: string[];
  known_constraints?: string[];
  unknowns?: string[];
  keywords?: string[] | null;
  link?: string | null;
};
export type ProductAnalysisResponse = {
  id: string;
  status:
    | "queued"
    | "running"
    | "waiting_for_user_input"
    | "blocked"
    | "failed"
    | "completed";
  current_stage: "product" | "audience" | "hypotheses";
  product_result: any | null;
  audience_result: any | null;
  hypothesis_result: any | null;
  audience_review: any | null;
  hypothesis_review: any | null;
};
export type ProductAnalysisStatusResponse = {
  id: string;
  status:
    | "queued"
    | "running"
    | "waiting_for_user_input"
    | "blocked"
    | "failed"
    | "completed";
  current_stage: "product" | "audience" | "hypotheses";
  product_result_available: boolean;
  audience_result_available: boolean;
  hypothesis_result_available: boolean;
  audience_review_available: boolean;
  hypothesis_review_available: boolean;
};
export type ProductAnalysisReviewStage = "audience" | "hypotheses";
export type ProductAnalysisReviewResponse = {
  stage: ProductAnalysisReviewStage;
  selected_candidate_ids: string[];
  custom_candidates: {
    [key: string]: any;
  }[];
  updated_at: string;
};
export type ProductAnalysisReviewRequest = {
  stage: ProductAnalysisReviewStage;
  selected_candidate_ids: string[];
  custom_candidates: {
    [key: string]: any;
  }[];
};
export type ProductFileResponse = {
  id: string;
  hypothesis_id: string;
  type: "product" | "brandbook" | "reference" | "other";
  filename: string;
  content_type: string;
  size: number;
  created_at: string;
};
export type PersonaQueryPersona = {
  id: string;
  source: string;
  source_row_index: number;
  dimensions: {
    [key: string]: string;
  };
};
export type PersonaQueryResponse = {
  matched_count: number;
  personas: PersonaQueryPersona[];
};
export type PersonaQueryRequest = {
  sample_size: number;
  seed?: number;
  sources?: string[];
  dimension_filters?: {
    [key: string]: string[];
  };
};
export const {
  useRegisterUserMutation,
  useLoginUserMutation,
  useGetYandexDirectConnectionQuery,
  useConnectYandexDirectMutation,
  useDisconnectYandexDirectMutation,
  useGetYandexDirectAuthorizationUrlQuery,
  useRunProductFeedbackSurveyMutation,
  useListProductFeedbackSurveyResultsQuery,
  useListHypothesesQuery,
  useCreateHypothesisMutation,
  useGetHypothesisQuery,
  useUpdateHypothesisMutation,
  useDeleteHypothesisMutation,
  useLaunchHypothesisMutation,
  useGetHypothesisLaunchStatusQuery,
  useStopHypothesisMutation,
  useGetHypothesisProductQuery,
  useCreateProductMutation,
  useUpdateProductMutation,
  useDeleteProductMutation,
  useGetProductAnalysisQuery,
  useStartProductAnalysisMutation,
  useGetProductAnalysisStatusQuery,
  useSaveProductAnalysisReviewMutation,
  useContinueProductAnalysisMutation,
  useListProductFilesQuery,
  useUploadProductFileMutation,
  useDeleteProductFileMutation,
  useQueryPersonasMutation,
} = injectedRtkApi;
