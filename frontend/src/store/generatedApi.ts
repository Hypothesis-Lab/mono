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
    listProductAudiences: build.query<
      ListProductAudiencesApiResponse,
      ListProductAudiencesApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/audiences`,
      }),
    }),
    createProductAudience: build.mutation<
      CreateProductAudienceApiResponse,
      CreateProductAudienceApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/product/${queryArg.id}/audiences`,
        method: "POST",
        body: queryArg.createAudienceRequest,
      }),
    }),
    getAudience: build.query<GetAudienceApiResponse, GetAudienceApiArg>({
      query: (queryArg) => ({ url: `/api/v1/audiences/${queryArg.id}` }),
    }),
    updateAudience: build.mutation<
      UpdateAudienceApiResponse,
      UpdateAudienceApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/audiences/${queryArg.id}`,
        method: "PATCH",
        body: queryArg.updateAudienceRequest,
      }),
    }),
    deleteAudience: build.mutation<
      DeleteAudienceApiResponse,
      DeleteAudienceApiArg
    >({
      query: (queryArg) => ({
        url: `/api/v1/audiences/${queryArg.id}`,
        method: "DELETE",
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
  /** status 200 launched hypothesis */ HypothesisResponse;
export type LaunchHypothesisApiArg = {
  id: string;
};
export type StopHypothesisApiResponse =
  /** status 200 stopped hypothesis */ HypothesisResponse;
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
export type ListProductAudiencesApiResponse =
  /** status 200 list of audiences */ AudienceResponse[];
export type ListProductAudiencesApiArg = {
  id: string;
};
export type CreateProductAudienceApiResponse =
  /** status 201 created audience */ AudienceResponse;
export type CreateProductAudienceApiArg = {
  id: string;
  createAudienceRequest: CreateAudienceRequest;
};
export type GetAudienceApiResponse =
  /** status 200 audience */ AudienceResponse;
export type GetAudienceApiArg = {
  id: string;
};
export type UpdateAudienceApiResponse =
  /** status 200 updated audience */ AudienceResponse;
export type UpdateAudienceApiArg = {
  id: string;
  updateAudienceRequest: UpdateAudienceRequest;
};
export type DeleteAudienceApiResponse = unknown;
export type DeleteAudienceApiArg = {
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
    /** The file to upload. Only .docx, .doc, .pdf, and .txt files are accepted. */
    file: Blob;
    type: "product" | "brandbook" | "reference" | "other";
  };
};
export type DeleteProductFileApiResponse = unknown;
export type DeleteProductFileApiArg = {
  id: string;
};
export type AuthTokenResponse = {
  token: string;
};
export type ErrDetail = {
  code: number;
  description: string;
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
  | "other";
export type ProductResponse = {
  id: string;
  name: string;
  description?: string | null;
  category: ProductCategory;
  keywords: string[];
  price?: string | null;
  link?: string | null;
  hypothesis_id: string;
};
export type CreateProductRequest = {
  name: string;
  description?: string | null;
  category: ProductCategory;
  keywords?: string[];
  price?: string | null;
  link?: string | null;
  hypothesis_id: string;
};
export type UpdateProductRequest = {
  name?: string | null;
  description?: string | null;
  category?: ProductCategory;
  keywords?: string[] | null;
  price?: string | null;
  link?: string | null;
};
export type AudienceGender = "men" | "women" | "all";
export type AudienceResponse = {
  id: string;
  product_id: string;
  description: string;
  age_from?: number | null;
  age_to?: number | null;
  gender?: AudienceGender;
  interests?: string | null;
  geography?: string | null;
  pain_points?: string | null;
  desires_motivation?: string | null;
  created_at?: string;
};
export type CreateAudienceRequest = {
  description: string;
  age_from?: number | null;
  age_to?: number | null;
  gender?: AudienceGender;
  interests?: string | null;
  geography?: string | null;
  pain_points?: string | null;
  desires_motivation?: string | null;
};
export type UpdateAudienceRequest = {
  description?: string | null;
  age_from?: number | null;
  age_to?: number | null;
  gender?: AudienceGender;
  interests?: string | null;
  geography?: string | null;
  pain_points?: string | null;
  desires_motivation?: string | null;
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
export const {
  useRegisterUserMutation,
  useLoginUserMutation,
  useListHypothesesQuery,
  useCreateHypothesisMutation,
  useGetHypothesisQuery,
  useUpdateHypothesisMutation,
  useDeleteHypothesisMutation,
  useLaunchHypothesisMutation,
  useStopHypothesisMutation,
  useGetHypothesisProductQuery,
  useCreateProductMutation,
  useUpdateProductMutation,
  useDeleteProductMutation,
  useListProductAudiencesQuery,
  useCreateProductAudienceMutation,
  useGetAudienceQuery,
  useUpdateAudienceMutation,
  useDeleteAudienceMutation,
  useListProductFilesQuery,
  useUploadProductFileMutation,
  useDeleteProductFileMutation,
} = injectedRtkApi;
