from app.pipelines.audiences.catalog import PERSONAS
from app.pipelines.audiences.models import InterviewQuestion, SyntheticPersona

QUESTION_AUDIENCES: tuple[SyntheticPersona, ...] = tuple(PERSONAS[:2])

QUESTIONS = [
    InterviewQuestion(
        question_id="Q01",
        dimension="задачи",
        text="Какие задачи вы решаете с помощью продукта в этих ситуациях?",
    ),
    InterviewQuestion(
        question_id="Q02",
        dimension="чувства",
        text="Что вы чувствуете, когда пользуетесь продуктом?",
    ),
    InterviewQuestion(
        question_id="Q03",
        dimension="желаемые_чувства",
        text="Что вы хотите чувствовать, когда пользуетесь продуктом?",
    ),
    InterviewQuestion(
        question_id="Q04",
        dimension="барьеры",
        text="Что мешает вам пользоваться продуктом в этих ситуациях?",
    ),
    InterviewQuestion(
        question_id="Q05",
        dimension="нерешенная_ситуация",
        text="Что вы чувствуете или что происходит, когда эта ситуация не решается?",
    ),
]

if len(QUESTION_AUDIENCES) != 2:
    raise RuntimeError("audience questions pipeline requires exactly two audiences")
