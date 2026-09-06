// Facade over the orval-generated typed client.
// Page components import from @/api instead of the old @/lib/api.
export * from "./generated/endpoints";
export { customInstance } from "./customAxios";
