import styled from 'styled-components'

export const Page = styled.main`
  max-width: 100%;
  margin: 40px auto;
  padding: 0 16px;

  @media (min-width: 640px) {
    max-width: 560px;
    margin: 60px auto;
    padding: 0 24px;
  }

  @media (min-width: 1024px) {
    max-width: 640px;
    margin: 80px auto;
    padding: 0 32px;
  }

  @media (min-width: 1280px) {
    padding: 0 40px;
  }
`

export const Title = styled.h1`
  font-size: 28px;
  margin-bottom: 24px;
  color: #111;
`

export const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: 16px;
`

export const Field = styled.label`
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 14px;
  color: #333;
`

export const Input = styled.input`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
`

export const TextArea = styled.textarea`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  resize: vertical;
`

export const AudiencesSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
  margin-top: 16px;
`

export const AudienceList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
`

export const AudienceGenerationStatus = styled.p`
  margin: 0;
  font-size: 14px;
  color: #888;
`

export const AudienceItem = styled.li`
  border: 1px solid #e2e2e2;
  border-radius: 6px;
  background: #fff;
`

export const AudienceDetails = styled.details`
  width: 100%;
`

export const AudienceSummary = styled.summary`
  display: block;
  padding: 10px 12px;
  cursor: pointer;

  &::-webkit-details-marker {
    display: none;
  }
`

export const AudienceInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
`

export const AudienceDescription = styled.span`
  font-size: 14px;
  font-weight: 600;
  color: #333;
  word-break: break-all;
`

export const AudienceMeta = styled.span`
  font-size: 12px;
  color: #888;
`

export const AudienceData = styled.pre`
  margin: 0;
  padding: 12px;
  border-top: 1px solid #e2e2e2;
  background: #f7f7f7;
  color: #333;
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
`

export const FilesSection = styled.section`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fafafa;
  margin-top: 16px;
`

export const SectionTitle = styled.h2`
  font-size: 18px;
  margin: 0;
  color: #111;
`

export const FileList = styled.ul`
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
`

export const FileItem = styled.li`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #e2e2e2;
  border-radius: 6px;
  background: #fff;
`

export const FileInfo = styled.div`
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
`

export const FileName = styled.span`
  font-size: 14px;
  font-weight: 600;
  color: #333;
  word-break: break-all;
`

export const FileMeta = styled.span`
  font-size: 12px;
  color: #888;
`

export const TypeBadge = styled.span<{ $type: 'product' | 'brandbook' | 'reference' | 'other' }>`
  padding: 2px 8px;
  border-radius: 10px;
  background: ${({ $type }) => ($type === 'brandbook' ? '#e3f2e8' : '#e8ecff')};
  color: ${({ $type }) => ($type === 'brandbook' ? '#1d6b3c' : '#33408f')};
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.4px;
`

export const UploadForm = styled.form`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 12px;
  border-top: 1px solid #e2e2e2;
`

export const FileInput = styled.input`
  font-size: 14px;
`

export const Select = styled.select`
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 6px;
  font-size: 14px;
  background: #fff;
`

export const ButtonGroup = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
`

export const Button = styled.button<{ $primary?: boolean; $danger?: boolean; $success?: boolean }>`
  padding: 10px 16px;
  border: 1px solid ${({ $danger, $success }) => ($danger ? '#c00' : $success ? '#2a8a2a' : '#ccc')};
  border-radius: 6px;
  background: ${({ $primary, $danger, $success }) =>
    $primary ? '#111' : $danger ? '#c00' : $success ? '#2a8a2a' : '#fff'};
  color: ${({ $primary, $danger, $success }) => ($primary || $danger || $success ? '#fff' : '#111')};
  font-size: 14px;
  cursor: pointer;

  &:hover:not(:disabled) {
    background: ${({ $primary, $danger, $success }) =>
      $primary ? '#333' : $danger ? '#a00' : $success ? '#237023' : '#f5f5f5'};
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
`

export const Message = styled.p`
  font-size: 16px;
  color: #666;
`

export const ErrorMessage = styled.p`
  font-size: 16px;
  color: #c00;
`
