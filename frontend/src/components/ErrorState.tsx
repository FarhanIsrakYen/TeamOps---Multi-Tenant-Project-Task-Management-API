import { apiMessage, apiRequestID } from "../lib/api";
import { Button } from "./Button";

export function ErrorState({
  error,
  retry,
}: {
  error: unknown;
  retry?(): void;
}) {
  const requestID = apiRequestID(error);
  return (
    <div className="error-state" role="alert">
      <strong>We couldn’t load this view</strong>
      <p>{apiMessage(error)}</p>
      {requestID && <small>Request ID: {requestID}</small>}
      {retry && (
        <Button variant="secondary" size="compact" onClick={retry}>
          Try again
        </Button>
      )}
    </div>
  );
}
