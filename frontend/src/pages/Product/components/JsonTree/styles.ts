import styled from 'styled-components'

export const Tree = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  border-top: 1px solid #e2e2e2;
  background: #f7f7f7;
  font-size: 12px;
  line-height: 1.5;
`

export const Node = styled.details`
  border-radius: 4px;

  &[open] > summary::before {
    transform: rotate(90deg);
  }
`

export const NodeSummary = styled.summary`
  position: relative;
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 8px 5px 24px;
  border-radius: 4px;
  cursor: pointer;
  list-style: none;

  &::-webkit-details-marker {
    display: none;
  }

  &::before {
    position: absolute;
    left: 9px;
    content: '›';
    color: #666;
    font-size: 18px;
    line-height: 1;
    transition: transform 120ms ease;
  }

  &:hover {
    background: #ececec;
  }

  &:focus-visible {
    outline: 2px solid #333;
    outline-offset: 2px;
  }
`

export const NodeChildren = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 2px 0 2px 16px;
  padding-left: 10px;
  border-left: 1px solid #d4d4d4;
`

export const Primitive = styled.div`
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 5px 8px;
  overflow-wrap: anywhere;
`

export const Key = styled.span`
  color: #33408f;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-weight: 600;
`

export const Separator = styled.span`
  color: #888;
`

export const Value = styled.span`
  min-width: 0;
  color: #333;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  white-space: pre-wrap;
`

export const CollectionSize = styled.span`
  color: #888;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
`

export const EmptyCollection = styled.span`
  padding: 4px 8px;
  color: #888;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
`
